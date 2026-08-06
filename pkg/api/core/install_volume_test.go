package core

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// dsm73PackageSetting is the verbatim `data` object returned by
// SYNO.Core.Package.Setting `get` on a DS1621+ running DSM 7.3.2-86009.
//
// It is captured rather than hand-built because the whole defect lives in what
// this payload does NOT contain: there is no `default_vol` key. A struct
// literal setting DefaultVol to "" would assert the same thing while quietly
// assuming the answer; unmarshalling the real response proves it.
const dsm73PackageSetting = `{
  "autoupdateall": true,
  "autoupdateimportant": true,
  "enable_autoupdate": true,
  "enable_dsm": true,
  "enable_email": true,
  "mailset": true,
  "show_disable_autoupdate": true,
  "trust_level": 0,
  "update_channel": true,
  "volume_count": 1,
  "volume_list": [
    {
      "desc": "",
      "display": "Volume 1 (Available capacity:  1265.38 GB )",
      "mount_point": "/volume1",
      "size_free": "1358691713024",
      "size_total": "4585197027328",
      "vol_desc": ""
    }
  ]
}`

func settingsFromJSON(t *testing.T, raw string) *PackageSettingGetResponse {
	t.Helper()
	var s PackageSettingGetResponse
	require.NoError(t, json.Unmarshal([]byte(raw), &s))
	return &s
}

// TestDSM73ReturnsNoDefaultVolume pins the root cause. Every install on this
// DSM version failed with "default volume empty" because the code read
// default_vol and nothing else; the volume was there the whole time, one field
// over.
func TestDSM73ReturnsNoDefaultVolume(t *testing.T) {
	s := settingsFromJSON(t, dsm73PackageSetting)

	require.Empty(t, s.DefaultVol,
		"DSM 7.3 does not return default_vol; if this ever becomes non-empty the "+
			"workaround below can be simplified")
	require.Len(t, s.VolumeList, 1, "the volume is present, under volume_list")
	require.Equal(t, "/volume1", s.VolumeList[0].MountPoint)
}

func TestResolveInstallVolume(t *testing.T) {
	twoVolumes := `{"volume_count":2,"volume_list":[
		{"mount_point":"/volume1"},{"mount_point":"/volume2"}]}`

	tests := []struct {
		name     string
		explicit string
		settings *PackageSettingGetResponse
		want     string
		wantErr  string
	}{
		{
			// Configuration beats inference: an explicit choice is never
			// second-guessed, even when DSM offers a default.
			name:     "explicit path wins over the DSM default",
			explicit: "/volume2",
			settings: settingsFromJSON(t, `{"default_vol":"/volume1",
				"volume_list":[{"mount_point":"/volume1"}]}`),
			want: "/volume2",
		},
		{
			name:     "explicit path wins when DSM offers nothing at all",
			explicit: "/volume3",
			settings: settingsFromJSON(t, `{}`),
			want:     "/volume3",
		},
		{
			name:     "DSM default is used when it is present",
			settings: settingsFromJSON(t, `{"default_vol":"/volume1"}`),
			want:     "/volume1",
		},
		{
			// The DSM 7.3 case, and the reason this function exists.
			name:     "sole volume is used when DSM returns no default",
			settings: settingsFromJSON(t, dsm73PackageSetting),
			want:     "/volume1",
		},
		{
			// One volume is not a guess -- there is nothing to get wrong.
			// Several is, so it is refused rather than picked.
			name:     "several volumes and no default is an error, not a guess",
			settings: settingsFromJSON(t, twoVolumes),
			wantErr:  "ambiguous install volume",
		},
		{
			name:     "the ambiguity error names the candidates",
			settings: settingsFromJSON(t, twoVolumes),
			wantErr:  "/volume1, /volume2",
		},
		{
			name:     "no volumes at all is an error",
			settings: settingsFromJSON(t, `{"volume_count":0,"volume_list":[]}`),
			wantErr:  "no install volume",
		},
		{
			// Entries without a mount point cannot be installed onto, so they
			// must not count toward "exactly one candidate".
			name: "volumes without a mount point are not candidates",
			settings: settingsFromJSON(t,
				`{"volume_list":[{"mount_point":""},{"mount_point":"/volume1"}]}`),
			want: "/volume1",
		},
		{
			name:     "nil settings is an error rather than a panic",
			settings: nil,
			wantErr:  "package settings unavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveInstallVolume(tt.explicit, tt.settings)
			if tt.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.wantErr)
				require.Empty(t, got, "no volume may be returned alongside an error")
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
