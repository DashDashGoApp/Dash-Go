package settings

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
)

// RuntimeFontSourceCommit is an immutable Google Fonts repository revision.
const RuntimeFontSourceCommit = "de88e79a24337aa0209f3abcc044d2500ca07021"

type RuntimeFontAsset struct {
	File, Family, Weight, URL, SHA256 string
}

type RuntimeFontSpec struct {
	Key, Family, Fallback string
	Assets                []RuntimeFontAsset
}

var runtimeFontSpecs = map[string]RuntimeFontSpec{
	"rounded": {
		Key: "rounded", Family: "Nunito", Fallback: "Libre Franklin",
		Assets: []RuntimeFontAsset{{
			File: "Nunito-variable.ttf", Family: "Nunito", Weight: "200 900",
			URL:    "https://fonts.gstatic.com/s/nunito/v32/XRXI3I6Li01BKofiOc5wtlZ2di8HDFwmRTM.ttf",
			SHA256: "1a025dfce6e6e03bbe31ba82277ec5ed96df8b8dd58b3df6267269490b90b1dc",
		}},
	},
	"readable": {
		Key: "readable", Family: "Atkinson Hyperlegible", Fallback: "DejaVu Sans",
		Assets: []RuntimeFontAsset{
			{File: "AtkinsonHyperlegible-Regular.ttf", Family: "Atkinson Hyperlegible", Weight: "400", URL: "https://raw.githubusercontent.com/google/fonts/" + RuntimeFontSourceCommit + "/ofl/atkinsonhyperlegible/AtkinsonHyperlegible-Regular.ttf", SHA256: "64024991d42cd9cddc09cd349e5305cbe537b2eb73cd014e95da1ab16b4a64f3"},
			{File: "AtkinsonHyperlegible-Bold.ttf", Family: "Atkinson Hyperlegible", Weight: "700", URL: "https://raw.githubusercontent.com/google/fonts/" + RuntimeFontSourceCommit + "/ofl/atkinsonhyperlegible/AtkinsonHyperlegible-Bold.ttf", SHA256: "6eb91bdb2d384bf462c8d012d86545e154423541e5abebd1fcb8205c767ea9e4"},
		},
	},
}

// RuntimeFontSpecs returns a defensive copy for diagnostics/tests. Production
// selection remains private so source integrity metadata cannot be mutated.
func RuntimeFontSpecs() map[string]RuntimeFontSpec {
	out := make(map[string]RuntimeFontSpec, len(runtimeFontSpecs))
	for key, spec := range runtimeFontSpecs {
		copySpec := spec
		copySpec.Assets = append([]RuntimeFontAsset(nil), spec.Assets...)
		out[key] = copySpec
	}
	return out
}

func (s *Service) FontStatus() map[string]any {
	required := []string{"LibreFranklin-400.ttf", "LibreFranklin-600.ttf", "LibreFranklin-700.ttf", "LibreFranklin-800.ttf", "DMMono-500.ttf"}
	missing := []string{}
	for _, name := range required {
		if !fileio.Exists(filepath.Join(s.bundledFontsDir, name)) {
			missing = append(missing, name)
		}
	}
	return map[string]any{"present": len(missing) == 0, "missing": missing, "dir": s.bundledFontsDir}
}

func (s *Service) RuntimeFontState(key string) string {
	switch key {
	case "system":
		return "system"
	case "default", "mono":
		if s.FontStatus()["present"] == true {
			return "bundled"
		}
		return "missing"
	}
	spec, ok := runtimeFontSpecs[key]
	if !ok {
		return "missing"
	}
	for _, asset := range spec.Assets {
		if !RuntimeFontAssetValid(filepath.Join(s.fontsDir, asset.File), asset) {
			return "missing"
		}
	}
	return "downloaded"
}

func (s *Service) FontStatusPayload() map[string]any {
	out := map[string]any{}
	for _, key := range []string{"system", "rounded", "default", "readable", "mono"} {
		_, downloadable := runtimeFontSpecs[key]
		out[key] = map[string]any{"state": s.RuntimeFontState(key), "downloadable": downloadable}
	}
	return out
}

// FontFaceCSS returns only verified user-downloaded font faces.
func (s *Service) FontFaceCSS() string {
	var b strings.Builder
	for _, spec := range runtimeFontSpecs {
		if s.RuntimeFontState(spec.Key) != "downloaded" {
			continue
		}
		for _, asset := range spec.Assets {
			fmt.Fprintf(&b, "@font-face{font-family:%q;font-style:normal;font-weight:%s;font-display:swap;src:url('/fonts/%s') format('truetype');}\n", asset.Family, asset.Weight, asset.File)
		}
	}
	return b.String()
}

// RuntimeFontPath validates a public font leaf name and returns a verified file
// path only when it is one of the pinned runtime assets.
func (s *Service) RuntimeFontPath(name string) (string, bool) {
	if filepath.Base(name) != name || name == "." || name == "" {
		return "", false
	}
	for _, spec := range runtimeFontSpecs {
		for _, asset := range spec.Assets {
			if asset.File == name {
				path := filepath.Join(s.fontsDir, name)
				return path, RuntimeFontAssetValid(path, asset)
			}
		}
	}
	return "", false
}

// DownloadRuntimeFontWithClient stages every asset and verifies full SHA-256
// content before any live font file is replaced.
func (s *Service) DownloadRuntimeFontWithClient(spec RuntimeFontSpec, client *http.Client) error {
	if client == nil {
		return fmt.Errorf("font download client is unavailable")
	}
	if err := os.MkdirAll(s.fontsDir, 0755); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(s.fontsDir, ".font-stage-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	for _, asset := range spec.Assets {
		if !ValidRuntimeFontSHA256(asset.SHA256) {
			return fmt.Errorf("font source integrity metadata is invalid")
		}
		req, err := http.NewRequest(http.MethodGet, asset.URL, nil)
		if err != nil {
			return err
		}
		req.Header.Set("User-Agent", "Dash-Go font download")
		res, err := client.Do(req)
		if err != nil {
			return err
		}
		if res.StatusCode != http.StatusOK {
			res.Body.Close()
			return fmt.Errorf("source returned %s", res.Status)
		}
		destination := filepath.Join(stage, asset.File)
		f, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
		if err == nil {
			_, err = io.Copy(f, io.LimitReader(res.Body, 2*1024*1024+1))
			closeErr := f.Close()
			if err == nil {
				err = closeErr
			}
		}
		res.Body.Close()
		if err != nil || !RuntimeFontAssetValid(destination, asset) {
			return fmt.Errorf("font integrity check failed")
		}
	}
	files := []map[string]any{}
	for _, asset := range spec.Assets {
		from, to := filepath.Join(stage, asset.File), filepath.Join(s.fontsDir, asset.File)
		if err := os.Rename(from, to); err != nil {
			return err
		}
		files = append(files, map[string]any{"file": asset.File, "family": asset.Family, "weight": asset.Weight, "sha256": asset.SHA256})
	}
	manifest, ok := readJSONMap(filepath.Join(s.fontsDir, "installed.json"))
	if !ok {
		manifest = map[string]any{}
	}
	manifest[spec.Key] = map[string]any{"family": spec.Family, "files": files, "installedAt": time.Now().Unix(), "sourceCommit": RuntimeFontSourceCommit}
	return fileio.WriteJSON(filepath.Join(s.fontsDir, "installed.json"), manifest)
}

func readJSONMap(path string) (map[string]any, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var value map[string]any
	if err := json.Unmarshal(b, &value); err != nil {
		return nil, false
	}
	return value, true
}

func FontLooksValid(path string) bool {
	b, err := os.ReadFile(path)
	if err != nil || len(b) < 4096 || len(b) > 2*1024*1024 {
		return false
	}
	if strings.Contains(strings.ToLower(string(b[:min(len(b), 160)])), "<html") || len(b) < 4 {
		return false
	}
	magic := string(b[:4])
	return magic == "OTTO" || magic == "true" || magic == "ttcf" || (b[0] == 0 && b[1] == 1 && b[2] == 0 && b[3] == 0)
}

func ValidRuntimeFontSHA256(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func RuntimeFontSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func RuntimeFontAssetValid(path string, asset RuntimeFontAsset) bool {
	if !ValidRuntimeFontSHA256(asset.SHA256) || !FontLooksValid(path) {
		return false
	}
	sum, err := RuntimeFontSHA256(path)
	return err == nil && strings.EqualFold(sum, asset.SHA256)
}
