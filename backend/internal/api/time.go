package api

import "time"

func nowRFC3339() string {
	return time.Now().Format(time.RFC3339)
}
