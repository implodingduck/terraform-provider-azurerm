package webapplicationfirewallpolicies

import "fmt"

const defaultApiVersion = "2020-04-01"

func userAgent() string {
	return fmt.Sprintf("pandora/webapplicationfirewallpolicies/%s", defaultApiVersion)
}
