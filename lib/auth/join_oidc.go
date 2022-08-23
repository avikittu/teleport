package auth

import (
	"context"
	"fmt"

	"github.com/coreos/go-oidc"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"
)

// example jwks endpoint
// https://www.googleapis.com/oauth2/v3/certs

type gcpIDToken struct {
	Sub    string `json:"sub"`
	Google struct {
		ComputeEngine struct {
			ProjectID     string `json:"project_id"`
			ProjectNumber int    `json:"project_number"`
			InstanceID    string `json:"instance_id"`
			InstanceName  string `json:"instance_name"`
		} `json:"compute_engine"`
	} `json:"google"`
}

// TODO: This is currently very GCP centric, extract GCP specific functionality
// to some kind of provider via an interface :D
func (a *Server) checkOIDCJoinRequest(ctx context.Context, req *types.RegisterUsingTokenRequest) error {
	if req.OIDCJWT == "" {
		return trace.BadParameter("Request must contain OIDCJWT value when using OIDC based joining.")
	}
	tokenResource, err := a.GetToken(ctx, req.Token)
	if err != nil {
		return trace.Wrap(err)
	}

	clusterName, err := a.GetClusterName()
	if err != nil {
		return trace.Wrap(err)
	}

	// TODO:
	// Instantiating this provider hits the issuer's well-known endpoint
	// and configures a cache for the JWKs, because of this, we probably
	// actually want to instantiate this elsewhere and inject this, rather than
	// instantiating this for every validation.
	provider, err := oidc.NewProvider(ctx, "https://accounts.google.com")
	if err != nil {
		return trace.Wrap(err)
	}

	v := provider.Verifier(&oidc.Config{
		// Expect the audience of the token to be the name of the cluster.
		// This reduces the risk that a JWT leaked by another application can
		// be used against Teleport.
		ClientID:             clusterName.GetClusterName(),
		SupportedSigningAlgs: []string{oidc.RS256, oidc.RS384, oidc.RS512},
		Now:                  a.GetClock().Now,
	})

	verifiedToken, err := v.Verify(ctx, req.OIDCJWT)
	if err != nil {
		return trace.Wrap(err)
	}

	var parsedClaims gcpIDToken
	if err := verifiedToken.Claims(&parsedClaims); err != nil {
		return trace.Wrap(err)
	}

	// If a single rule passes, accept the token
	for _, rule := range tokenResource.GetAllowRules() {
		if rule.Sub != "" && rule.Sub != parsedClaims.Sub {
			continue
		}

		if rule.Google != nil && rule.Google.ComputeEngine != nil {
			want := rule.Google.ComputeEngine
			is := parsedClaims.Google.ComputeEngine
			if want.ProjectID != "" && want.ProjectID != is.ProjectID {
				continue
			}
			if want.ProjectNumber != 0 && int(want.ProjectNumber) != is.ProjectNumber {
				continue
			}
			if want.InstanceID != "" && want.InstanceID != is.InstanceID {
				continue
			}
			if want.InstanceName != "" && want.InstanceName != is.InstanceName {
				continue
			}
		}

		// The rule passed, so we should return without error
		return nil
	}

	// TODO: Make this error more useful
	return fmt.Errorf("token did not match any allow rules")
}
