package tool

import (
	"crypto/sha256"
	"sync"

	"github.com/flosch/pongo2/v6"
)

type compiled_template struct {
	hash     [32]byte
	template *pongo2.Template
}

var template_cache_lock sync.Mutex
var template_cache = map[string]compiled_template{}

// 내용이 바뀐 외부 스킨은 즉시 다시 읽고, 동일한 템플릿은 재사용한다.
// 사용자별 HTML과 권한 정보는 캐시에 넣지 않는다.
func load_template(name string) (*pongo2.Template, error) {
	data, err := Read_view_file(name)
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(data)
	template_cache_lock.Lock()
	defer template_cache_lock.Unlock()
	if cached, ok := template_cache[name]; ok && cached.hash == hash {
		return cached.template, nil
	}
	template, err := pongo2.FromString(string(data))
	if err != nil {
		return nil, err
	}
	if len(template_cache) >= 32 {
		template_cache = map[string]compiled_template{}
	}
	template_cache[name] = compiled_template{hash: hash, template: template}
	return template, nil
}
