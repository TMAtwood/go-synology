package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// installError4501 is the verbatim body DSM 7.3.2-86009 returns from
// SYNO.Core.Package.Installation `install` when the install fails.
//
// It is captured rather than invented because the defect is entirely in its
// SHAPE: `errors` is an object here, while other endpoints return an array.
// The typed ApiError accepted only the array, so this body failed to decode,
// and the failure was then reported to users as "OTP code is required by the
// server" -- on an account with no second factor configured. The real error
// code never reached the caller.
const installError4501 = `{"error":{"code":4501,"errors":{` +
	`"packageName":"/volume1/@tmp/synopkg/download.mWN4sc/@SYNOPKG_DOWNLOAD_MariaDB10",` +
	`"worker_message":null}},"success":false}`

type probeData struct {
	TaskID string `json:"taskid"`
}

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

// TestApiErrorAcceptsObjectShapedErrors is the root-cause test. DSM returns
// `errors` both ways and the client has to survive both.
func TestApiErrorAcceptsObjectShapedErrors(t *testing.T) {
	var resp ApiResponse[probeData]
	require.NoError(t, json.Unmarshal([]byte(installError4501), &resp),
		"the object form of `errors` must decode; refusing it is what caused a real "+
			"DSM error to be reported as an authentication failure")

	require.False(t, resp.Success)
	require.Equal(t, 4501, resp.Error.Code,
		"the error code is the only part a caller can act on and must survive decoding")
}

func TestApiErrorAcceptsArrayShapedErrors(t *testing.T) {
	// The shape the struct was originally written for. It must keep working.
	const body = `{"error":{"code":119,"errors":[{"code":1,"reason":"x"}]},"success":false}`

	var resp ApiResponse[probeData]
	require.NoError(t, json.Unmarshal([]byte(body), &resp))
	require.Equal(t, 119, resp.Error.Code)
	require.Len(t, resp.Error.Errors, 1)
}

func TestApiErrorSurvivesAnUnrecognisedErrorsShape(t *testing.T) {
	// A third shape must still yield the code. Discarding the whole response to
	// preserve detail nobody can read is the trade that produced the bug.
	const body = `{"error":{"code":4501,"errors":"a bare string"},"success":false}`

	var resp ApiResponse[probeData]
	require.NoError(t, json.Unmarshal([]byte(body), &resp))
	require.Equal(t, 4501, resp.Error.Code)
}

// TestHandleDoesNotReportUnexpectedShapesAsOtp guards the second half of the
// defect. ApiResponsePartialAuth has no `data` field, so nearly any DSM JSON
// decodes into it; treating that as proof of an OTP challenge turned the
// fallback into a catch-all that mislabelled every unexpected shape.
func TestHandleDoesNotReportUnexpectedShapesAsOtp(t *testing.T) {
	// `data` is an array where probeData is an object: decodes as partial auth,
	// but is not remotely an OTP challenge.
	const mismatched = `{"success":true,"data":["not","an","object"]}`

	_, err := handle[probeData](jsonResponse(mismatched), GlobalErrors)

	require.Error(t, err)
	require.NotErrorIs(t, err, ErrOtpRequired,
		"an unexpected response shape is not an authentication failure; saying so "+
			"sends the reader to look at 2FA on an account that has none")
	require.Contains(t, err.Error(), "unexpected response shape",
		"the error must describe what actually happened")
}

// TestHandleStillReportsARealOtpChallenge is the other half: the carve-out
// above must not silence a genuine challenge.
func TestHandleStillReportsARealOtpChallenge(t *testing.T) {
	// DSM names the required factor types and hands back a one-time token.
	const challenge = `{"success":false,"error":{"code":403,"errors":{` +
		`"token":"abc123","types":[{"type":"otp"}]}}}`

	_, err := handle[probeData](jsonResponse(challenge), GlobalErrors)

	require.ErrorIs(t, err, ErrOtpRequired)
}

func TestIsOtpChallenge(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{
			name: "types naming otp",
			body: `{"success":false,"error":{"code":403,"errors":{"types":[{"type":"otp"}]}}}`,
			want: true,
		},
		{
			name: "a one-time token alone is enough",
			body: `{"success":false,"error":{"code":403,"errors":{"token":"abc"}}}`,
			want: true,
		},
		{
			name: "an unrelated failure is not a challenge",
			body: `{"success":false,"error":{"code":4501,"errors":{}}}`,
			want: false,
		},
		{
			// The case that made the old branch a catch-all: a SUCCESSFUL
			// response decodes into ApiResponsePartialAuth perfectly well.
			name: "a successful response is never a challenge",
			body: `{"success":true}`,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var r ApiResponsePartialAuth[probeData]
			require.NoError(t, json.Unmarshal([]byte(tt.body), &r))
			require.Equal(t, tt.want, isOtpChallenge(r))
		})
	}
}
