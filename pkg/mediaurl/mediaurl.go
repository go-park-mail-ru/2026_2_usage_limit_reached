package mediaurl

import "strings"

func Public(publicPath, key string) string {
	return strings.TrimSuffix(publicPath, "/") + "/" + key
}

func PublicPtr(publicPath string, key *string) *string {
	if key == nil {
		return nil
	}

	u := Public(publicPath, *key)

	return &u
}
