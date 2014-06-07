package files

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
)

// JSONFileToInterface reads a JSON file at a given path and populates the given
// interface with the contents
func JSONFileToInterface(path string, v interface{}) error {
	if path == "" {
		return fmt.Errorf("path was empty")
	}

	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}

	err = json.Unmarshal(bytes, &v)
	if err != nil {
		return err
	}

	return nil
}
