package cli

import (
	"encoding/json"
	"io/fs"

	hm "github.com/hiberus-magento/hiberus-dockergento"
)

// toolAssets is the files the tool ships with: the copy compiled into the binary, or the
// directory HM_ASSETS_DIR names. A variable so that a test can hand its own.
var toolAssets = hm.Assets

// readAssetObject is one of the tool's JSON files, by its slash-separated name.
func readAssetObject(name string) (map[string]any, error) {
	contents, err := fs.ReadFile(toolAssets(), name)
	if err != nil {
		return nil, err
	}

	var object map[string]any
	if err := json.Unmarshal(contents, &object); err != nil {
		return nil, err
	}

	return object, nil
}
