package provider

import (
	"errors"

	"github.com/rusq/slackdump/v3/auth"
	"go.uber.org/zap"
)

// ValidateTokens checks that the given Slack credentials authenticate against
// the workspace and returns its TeamID. Priority mirrors New(): xoxp > xoxb >
// xoxc/xoxd. It returns an error instead of calling logger.Fatal, so callers
// (e.g. `slack-cli auth login`) can handle failures gracefully.
//
// Lives in a separate file (additive) to keep the rest of the provider package
// identical to upstream for clean merges.
func ValidateTokens(xoxp, xoxb, xoxc, xoxd string, logger *zap.Logger) (string, error) {
	var (
		ap  auth.ValueAuth
		err error
	)
	switch {
	case xoxp != "":
		ap, err = auth.NewValueAuth(xoxp, "")
	case xoxb != "":
		ap, err = auth.NewValueAuth(xoxb, "")
	case xoxc != "" && xoxd != "":
		ap, err = auth.NewValueAuth(xoxc, xoxd)
	default:
		return "", errors.New("no credentials: provide xoxp, xoxb, or xoxc+xoxd")
	}
	if err != nil {
		return "", err
	}
	return validateAuthAndGetTeamID(ap, logger)
}
