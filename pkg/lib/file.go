package lib

import "os"

func LoadJsonFile(path string, value interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return Json.Unmarshal(data, value)
}
