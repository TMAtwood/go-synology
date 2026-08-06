package core

import (
	"encoding/json"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

// DSM expects `extra_values` as a JSON *string* -- quoted, with inner quotes
// escaped -- not as a bare JSON object. Getting that wrong is expensive to
// diagnose because neither endpoint says so:
//
//   - SYNO.Core.Package.Installation answers a malformed value with the
//     generic error 4501, naming nothing.
//   - SYNO.Core.Package.Uninstallation answers a bare object with error 120
//     and {"name":"extra_values","reason":"type"}, which at least names the
//     parameter.
//
// Both shapes were wrong in this client, in opposite directions, and the
// install side was masked by its own empty case -- see below.

// decodeExtraValues asserts the wire value is a JSON string, and returns the
// object it encodes. It fails on a bare object, which is the shape DSM rejects.
func decodeExtraValues(t *testing.T, wire string) map[string]any {
	t.Helper()

	var inner string
	require.NoError(t, json.Unmarshal([]byte(wire), &inner),
		"extra_values must be a JSON string: DSM rejects a bare object. got %s", wire)

	var obj map[string]any
	require.NoError(t, json.Unmarshal([]byte(inner), &obj),
		"the string must itself contain a JSON object. got %s", inner)
	return obj
}

func TestExtraValuesEncodesAsAQuotedJSONString(t *testing.T) {
	v := url.Values{}
	require.NoError(t, ExtraValues{
		"pkgwizard_port":              "3306",
		"pkgwizard_new_root_password": `a"quote`,
	}.EncodeValues("extra_values", &v))

	obj := decodeExtraValues(t, v.Get("extra_values"))
	require.Equal(t, "3306", obj["pkgwizard_port"])
	require.Equal(t, `a"quote`, obj["pkgwizard_new_root_password"],
		"a value containing a quote must survive; unescaped wrapping loses it")
}

// TestExtraValuesEmptyCaseMaskedTheBug is the regression that matters most.
//
// The empty map emitted `"{}"`, which is *already* correctly quoted, so every
// package without a wizard installed fine. Only packages WITH a wizard hit the
// unescaped path. The symptom therefore looked like "the provider cannot
// install packages" rather than "the wizard encoding is broken", and a test
// covering only the empty case would have passed throughout.
func TestExtraValuesEmptyCaseMaskedTheBug(t *testing.T) {
	empty := url.Values{}
	require.NoError(t, ExtraValues{}.EncodeValues("extra_values", &empty))
	require.Empty(t, decodeExtraValues(t, empty.Get("extra_values")))

	populated := url.Values{}
	require.NoError(t, ExtraValues{"k": "v"}.EncodeValues("extra_values", &populated))
	require.NotEmpty(t, decodeExtraValues(t, populated.Get("extra_values")))

	// Both must be the same KIND of value. The bug was that they were not:
	// the empty case produced a valid JSON string and the populated case did
	// not, so the two paths disagreed about the wire format.
	require.Equal(t, byte('"'), empty.Get("extra_values")[0])
	require.Equal(t, byte('"'), populated.Get("extra_values")[0],
		"populated extra_values must be quoted like the empty case")
}

func TestUninstallExtraEncodesAsAQuotedJSONString(t *testing.T) {
	// Measured against the device: a bare object is refused with error 120,
	// {"name":"extra_values","reason":"type"}. The quoted form uninstalls.
	v := url.Values{}
	require.NoError(t, UninstallExtra{KeepData: true}.EncodeValues("extra_values", &v))

	obj := decodeExtraValues(t, v.Get("extra_values"))
	require.Equal(t, true, obj["wizard_keep_data"])
	require.Equal(t, false, obj["wizard_delete_data"])
}
