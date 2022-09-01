package azsessions

import (
	"context"
	"net/url"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/events/test"
	"github.com/gravitational/teleport/lib/utils"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

func TestStreams(t *testing.T) {
	ctx := context.Background()

	envURL := os.Getenv(teleport.AZBlobTestURI)
	if envURL == "" {
		t.Skipf("Skipping azsessions tests as %q is not set.", teleport.AZBlobTestURI)
	}

	u, err := url.Parse(envURL)
	require.NoError(t, err)

	handler, err := NewHandlerFromURL(ctx, u)
	require.Nil(t, err)

	t.Run("StreamSinglePart", func(t *testing.T) {
		test.StreamSinglePart(t, handler)
	})
	t.Run("UploadDownload", func(t *testing.T) {
		test.UploadDownload(t, handler)
	})
	t.Run("DownloadNotFound", func(t *testing.T) {
		test.DownloadNotFound(t, handler)
	})
}
