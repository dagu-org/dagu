package sock

import (
	"crypto/md5"
	"fmt"
	"path"
	"strings"
)

const sockDir = "/tmp"

func GetSockAddr(key string) string {
	s := strings.ReplaceAll(key, " ", "_")
	name := strings.Replace(path.Base(s), path.Ext(path.Base(s)), "", 1)
	h := md5.New()
	h.Write([]byte(s))
	bs := h.Sum(nil)
	return path.Join(sockDir, fmt.Sprintf("@dagu-%s-%x", name, bs))
}
