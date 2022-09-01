package azsessions

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/url"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/streaming"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport"

	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"
)

var eTagAny = azblob.ETagAny

var blobDoesNotExist = azblob.BlobAccessConditions{
	ModifiedAccessConditions: &azblob.ModifiedAccessConditions{
		IfNoneMatch: &eTagAny,
	},
}

const (
	sessionContainerName    = "session"
	inprogressContainerName = "inprogress"

	// uploadMarkerPrefix is the prefix for upload markers, stored at `upload/<session ID>/<upload ID>`.
	uploadMarkerPrefix = "upload/"
	// uploadMarkerFmt is the format string for upload markers, stored at `upload/<session ID>/<upload ID>`.
	uploadMarkerFmt = "upload/%v/%v"

	// partFmt is the format string for upload parts, stored at `part/<session ID>/<upload ID>/<part number>`.
	partFmt = "part/%v/%v/%v"

	// clientIDFragParam is the parameter in the fragment that specifies the optional client ID.
	clientIDFragParam = "azure_client_id"
)

// cErr attempts to convert err to a meaningful trace error; if it can't, it'll
// return the error, wrapped.
func cErr(err error) error {
	var stErr *azblob.StorageError
	if !errors.As(err, &stErr) || stErr == nil {
		return trace.Wrap(err)
	}

	return trace.WrapWithMessage(trace.ReadError(stErr.StatusCode(), nil), stErr.ErrorCode)
}

// cErr2 discards the first argument and attempts to convert err to a meaningful
// trace error; if it can't, it'll return the error, wrapped.
func cErr2(_ any, err error) error {
	return cErr(err)
}

type Config struct {
	// ServiceURL is the URL for the storage account to use.
	ServiceURL url.URL

	// ClientID, when set, defines the managed identity's client ID to use for
	// authentication.
	ClientID string

	// Log is the logger to use. If unset, it will default to the global logger
	// with a component of "azblob".
	Log logrus.FieldLogger
}

func (c *Config) SetFromURL(u *url.URL) error {
	c.ServiceURL = *u

	switch c.ServiceURL.Scheme {
	case teleport.SchemeAZBlob:
		c.ServiceURL.Scheme = "https"
	case teleport.SchemeAZBlobHTTP:
		c.ServiceURL.Scheme = "http"
	}

	params, _ := url.ParseQuery(c.ServiceURL.EscapedFragment())
	c.ServiceURL.Fragment = ""
	c.ServiceURL.RawFragment = ""

	c.ClientID = params.Get(clientIDFragParam)

	return nil
}

func (c *Config) CheckAndSetDefaults() error {
	if c.Log == nil {
		c.Log = logrus.WithField(trace.Component, teleport.SchemeAZBlob)
	}

	return nil
}

func NewHandlerFromURL(ctx context.Context, u *url.URL) (*Handler, error) {
	var cfg Config
	cfg.SetFromURL(u)
	return NewHandler(ctx, cfg)
}

func NewHandler(ctx context.Context, cfg Config) (*Handler, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, err
	}

	var cred azcore.TokenCredential
	if cfg.ClientID != "" {
		c, err := azidentity.NewManagedIdentityCredential(&azidentity.ManagedIdentityCredentialOptions{
			ID: azidentity.ClientID(cfg.ClientID),
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		cred = c
	} else {
		c, err := azidentity.NewDefaultAzureCredential(nil)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		cred = c
	}

	cred = &cachedTokenCredential{TokenCredential: cred}

	service, err := azblob.NewServiceClient(cfg.ServiceURL.String(), cred, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	session, err := service.NewContainerClient(sessionContainerName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	inprogress, err := service.NewContainerClient(inprogressContainerName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO: warn on permission errors, we might not have permissions to create containers
	if err := cErr2(session.Create(ctx, nil)); err != nil && !trace.IsAlreadyExists(err) {
		return nil, err
	}
	if err := cErr2(inprogress.Create(ctx, nil)); err != nil && !trace.IsAlreadyExists(err) {
		return nil, err
	}
	cfg.Log.Debug("successfully created containers")

	return &Handler{c: cfg, cred: cred, session: session, inprogress: inprogress}, nil
}

type Handler struct {
	c          Config
	cred       azcore.TokenCredential
	session    *azblob.ContainerClient
	inprogress *azblob.ContainerClient
}

var _ events.MultipartHandler = (*Handler)(nil)

func (h *Handler) sessionBlob(sessionID session.ID) (*azblob.BlockBlobClient, error) {
	blobName := sessionID.String()
	client, err := h.session.NewBlockBlobClient(blobName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return client, nil
}

func (h *Handler) uploadMarkerBlob(upload events.StreamUpload) (*azblob.BlockBlobClient, error) {
	blobName := fmt.Sprintf(uploadMarkerFmt, upload.SessionID, upload.ID)
	client, err := h.inprogress.NewBlockBlobClient(blobName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return client, nil
}

func (h *Handler) partBlob(upload events.StreamUpload, partNumber int64) (*azblob.BlockBlobClient, error) {
	blobName := fmt.Sprintf(partFmt, upload.SessionID, upload.ID, partNumber)
	client, err := h.inprogress.NewBlockBlobClient(blobName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return client, nil
}

func (h *Handler) Upload(ctx context.Context, sessionID session.ID, reader io.Reader) (string, error) {
	blob, err := h.sessionBlob(sessionID)
	if err != nil {
		return "", err
	}

	if err := cErr2(blob.UploadStream(ctx, reader, azblob.UploadStreamOptions{
		BlobAccessConditions: &blobDoesNotExist,
	})); err != nil {
		return "", err
	}

	return blob.URL(), nil
}

func (h *Handler) Download(ctx context.Context, sessionID session.ID, writer io.WriterAt) error {
	blob, err := h.sessionBlob(sessionID)
	if err != nil {
		return err
	}

	const beginOffset = 0
	return cErr(blob.DownloadToWriterAt(ctx, beginOffset, azblob.CountToEnd, writer, azblob.DownloadOptions{}))
}

func (h *Handler) CreateUpload(ctx context.Context, sessionID session.ID) (*events.StreamUpload, error) {
	upload := events.StreamUpload{
		ID:        uuid.NewString(),
		SessionID: sessionID,
	}

	blob, err := h.uploadMarkerBlob(upload)
	if err != nil {
		return nil, err
	}

	// a zero byte blob can be uploaded in one chunk, so we don't need UploadStream
	emptyBody := streaming.NopCloser(&bytes.Reader{})

	if err := cErr2(blob.Upload(ctx, emptyBody, &azblob.BlockBlobUploadOptions{
		BlobAccessConditions: &blobDoesNotExist,
	})); err != nil {
		return nil, err
	}

	return &upload, nil
}

func (h *Handler) CompleteUpload(ctx context.Context, upload events.StreamUpload, parts []events.StreamPart) error {
	blob, err := h.sessionBlob(upload.SessionID)
	if err != nil {
		return err
	}

	upBlob, err := h.uploadMarkerBlob(upload)
	if err != nil {
		return err
	}

	parts = append([]events.StreamPart(nil), parts...)
	sort.Slice(parts, func(i, j int) bool { return parts[i].Number < parts[j].Number })

	urls := make([]string, 0, len(parts))
	for _, part := range parts {
		b, err := h.partBlob(upload, part.Number)
		if err != nil {
			return err
		}
		urls = append(urls, b.URL())
	}

	token, err := h.cred.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: []string{"https://storage.azure.com/.default"},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	copySourceAuthorization := "Bearer " + token.Token

	eg, egCtx := errgroup.WithContext(ctx)
	eg.SetLimit(5)

	blocks := make([]string, len(urls))
	for i, url := range urls {
		i, url := i, url
		eg.Go(func() error {
			// we use block names that are local to this function so we don't
			// interact with other ongoing uploads; trick copied from
			// (*BlockBlobClient).UploadBuffer and UploadFile
			u := uuid.New()
			blocks[i] = base64.StdEncoding.EncodeToString(u[:])
			const contentLength = 0 // required by the API to be zero
			return cErr2(blob.StageBlockFromURL(egCtx, blocks[i], url, contentLength, &azblob.BlockBlobStageBlockFromURLOptions{
				CopySourceAuthorization: &copySourceAuthorization,
			}))
		})
	}
	if err := eg.Wait(); err != nil {
		return err
	}

	if err := cErr2(blob.CommitBlockList(ctx, blocks, &azblob.BlockBlobCommitBlockListOptions{
		BlobAccessConditions: &blobDoesNotExist,
	})); err != nil {
		if trace.IsAlreadyExists(err) {
			return nil
		}
		return err
	}

	upBlob.Delete(ctx, nil)

	// TODO: batch deletes
	for _, part := range parts {
		b, err := h.partBlob(upload, part.Number)
		if err != nil {
			continue
		}
		b.Delete(ctx, nil)
	}

	return nil
}

func (*Handler) ReserveUploadPart(ctx context.Context, upload events.StreamUpload, partNumber int64) error {
	return nil
}

func (h *Handler) UploadPart(ctx context.Context, upload events.StreamUpload, partNumber int64, partBody io.ReadSeeker) (*events.StreamPart, error) {
	blob, err := h.partBlob(upload, partNumber)
	if err != nil {
		return nil, err
	}

	if err := cErr2(blob.UploadStream(ctx, partBody, azblob.UploadStreamOptions{})); err != nil {
		return nil, err
	}

	return &events.StreamPart{Number: partNumber}, nil
}

func (h *Handler) ListParts(ctx context.Context, upload events.StreamUpload) ([]events.StreamPart, error) {
	prefix := fmt.Sprintf(partFmt, upload.SessionID, upload.ID, "")

	pager := h.inprogress.ListBlobsFlat(&azblob.ContainerListBlobsFlatOptions{
		Prefix: &prefix,
	})
	var parts []events.StreamPart
	for pager.NextPage(ctx) {
		resp := pager.PageResponse()
		if resp.Segment == nil {
			continue
		}
		for _, b := range resp.Segment.BlobItems {
			if b == nil || b.Name == nil || !strings.HasPrefix(*b.Name, prefix) {
				continue
			}
			pn := strings.TrimPrefix(*b.Name, prefix)
			partNumber, err := strconv.ParseInt(pn, 10, 0)
			if err != nil {
				continue
			}
			parts = append(parts, events.StreamPart{Number: partNumber})
		}
	}
	if err := pager.Err(); err != nil {
		return nil, cErr(err)
	}

	sort.Slice(parts, func(i, j int) bool { return parts[i].Number < parts[j].Number })

	return parts, nil
}

func (h *Handler) ListUploads(ctx context.Context) ([]events.StreamUpload, error) {
	prefix := uploadMarkerPrefix
	pager := h.inprogress.ListBlobsFlat(&azblob.ContainerListBlobsFlatOptions{
		Prefix: &prefix,
	})
	var uploads []events.StreamUpload
	for pager.NextPage(ctx) {
		r := pager.PageResponse()
		if r.Segment == nil {
			continue
		}
		for _, b := range r.Segment.BlobItems {
			if b == nil || b.Name == nil || !strings.HasPrefix(*b.Name, prefix) {
				continue
			}
			if b.Properties == nil || b.Properties.CreationTime == nil {
				continue
			}
			name := strings.TrimPrefix(*b.Name, prefix)
			sid, uid, ok := strings.Cut(name, "/")
			if !ok {
				continue
			}
			if _, err := session.ParseID(sid); err != nil {
				continue
			}
			if _, err := uuid.Parse(uid); err != nil {
				continue
			}

			uploads = append(uploads, events.StreamUpload{
				ID:        uid,
				SessionID: session.ID(sid),
				Initiated: *b.Properties.CreationTime,
			})
		}
	}
	if err := pager.Err(); err != nil {
		return nil, cErr(err)
	}

	sort.Slice(uploads, func(i, j int) bool { return uploads[i].Initiated.Before(uploads[j].Initiated) })

	return uploads, nil
}

func (h *Handler) GetUploadMetadata(sessionID session.ID) events.UploadMetadata {
	return events.UploadMetadata{
		URL:       h.c.ServiceURL.JoinPath(sessionContainerName, sessionID.String()).String(),
		SessionID: sessionID,
	}
}

type cachedTokenCredential struct {
	azcore.TokenCredential

	mu      sync.Mutex
	options policy.TokenRequestOptions
	token   azcore.AccessToken
}

var _ azcore.TokenCredential = (*cachedTokenCredential)(nil)

func (c *cachedTokenCredential) GetToken(ctx context.Context, options policy.TokenRequestOptions) (azcore.AccessToken, error) {
	c.mu.Lock()
	if reflect.DeepEqual(options, c.options) && c.token.ExpiresOn.After(time.Now().Add(10*time.Minute)) {
		defer c.mu.Unlock()
		return c.token, nil
	}
	c.mu.Unlock()

	token, err := c.TokenCredential.GetToken(ctx, options)
	if err != nil {
		return azcore.AccessToken{}, err
	}

	c.mu.Lock()
	c.options, c.token = options, token
	c.mu.Unlock()

	return token, nil
}
