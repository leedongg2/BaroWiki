package route

import (
	"opennamu/route/tool"
)

func View_edit_post(config tool.Config, doc_name string, data string, send string, agree string, captcha string, expected_revision string, section string) string {
	db := tool.DB_connect()
	defer tool.DB_close(db)
	if !tool.Captcha_check(db, config.Session, config.IP, captcha) {
		return tool.Get_error_page(db, config, "recaptcha")
	}

	// 문단 편집이면 원문에서 해당 문단만 교체한다
	if section != "" {
		raw_data := Api_w_raw(config, doc_name, "", "")
		if raw_data["response"].(string) != "ok" {
			return tool.Get_error_page(db, config, "not found")
		}

		merged_data, merged_ok := tool.Section_replace(raw_data["data"].(string), section, data)
		if !merged_ok {
			return tool.Get_error_page(db, config, "invalid data")
		}

		data = merged_data
	}

	return_data := Api_edit_post(config, doc_name, data, send, agree, expected_revision)

	result_html := ""
	if return_data["response"].(string) == "ok" {
		result_html = tool.Get_redirect("/w/" + tool.Url_parser(doc_name))
	} else if return_data["data"].(string) == "edit conflict" {
		// 충돌은 오류로 끝내지 않고 3자 병합 화면으로 넘긴다 (3-3)
		result_html = View_baro_conflict(config, doc_name, data, send, expected_revision)
	} else {
		result_html = tool.Get_error_page(db, config, return_data["data"].(string))
	}

	return result_html
}
