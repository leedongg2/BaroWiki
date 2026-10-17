package main

import (
	"net/http"
	"strconv"
	"strings"

	"opennamu/route"
	"opennamu/route/tool"

	"github.com/gin-gonic/gin"
)

// rev_resolve : 판 자리에 판 고유 번호가 왔으면 판 번호로 바꾼다
//
//	`/render/3/문서` 처럼 판 번호를 그대로 쓸 수도 있고
//	`/render/<고유 번호>/문서` 처럼 판 고유 번호를 쓸 수도 있다.
//	고유 번호가 아니면 받은 값을 그대로 돌려주므로 기존 주소는 그대로 쓰인다.
func rev_resolve(doc_name string, rev string) string {
	if !tool.Baro_uuid_is(rev) {
		return rev
	}

	db := tool.DB_connect()
	defer tool.DB_close(db)

	return tool.Baro_rev_resolve(db, doc_name, rev)
}

// rev_param : 판 자리 + `?uuid=` 를 함께 받는다 (판 하나만 가리키는 주소용)
//
//	diff 는 history_diff_revisions 에서 `uuid` 와 `olduuid` 를 따로 읽는다.
func rev_param(c *gin.Context, doc_name string, rev string) string {
	if query_uuid := c.Query("uuid"); tool.Baro_uuid_is(query_uuid) {
		rev = query_uuid
	}

	return rev_resolve(doc_name, rev)
}

func history_doc_revision(c *gin.Context, path string) (string, string) {
	path = strings.TrimPrefix(path, "/")
	if uuid := c.Query("uuid"); tool.Baro_uuid_is(uuid) {
		return path, rev_resolve(path, uuid)
	}

	first, rest, found := strings.Cut(path, "/")
	if found && (tool.Baro_uuid_is(first) || tool.Str_to_int(first) > 0) {
		return rest, rev_resolve(rest, first)
	}

	return path, ""
}

func history_previous_revision(doc_name string, revision string) string {
	if doc_name == "" || tool.Str_to_int(revision) <= 0 {
		return ""
	}

	db := tool.DB_connect()
	defer tool.DB_close(db)

	previous := ""
	tool.QueryRow_DB(
		db,
		"select id from history where title = ? and id + 0 > 0 and id + 0 < ? order by id + 0 desc limit 1",
		[]any{&previous},
		doc_name,
		tool.Str_to_int(revision),
	)

	return previous
}

func history_diff_revisions(c *gin.Context, path string) (string, string, string) {
	path = strings.TrimPrefix(path, "/")
	if uuid := c.Query("uuid"); tool.Baro_uuid_is(uuid) {
		doc_name := path
		after := rev_resolve(doc_name, uuid)
		before := ""
		if old_uuid := c.Query("olduuid"); tool.Baro_uuid_is(old_uuid) {
			before = rev_resolve(doc_name, old_uuid)
		} else {
			before = history_previous_revision(doc_name, after)
		}
		return doc_name, before, after
	}

	parts := strings.SplitN(path, "/", 3)
	if len(parts) != 3 {
		return "", "", ""
	}
	doc_name := parts[2]
	return doc_name, rev_resolve(doc_name, parts[0]), rev_resolve(doc_name, parts[1])
}

func history_revision_jump(c *gin.Context, doc_name string) string {
	revision_text := strings.TrimSpace(c.Query("rev"))
	revision_text = strings.TrimPrefix(strings.ToLower(revision_text), "r")
	revision, err := strconv.Atoi(revision_text)
	if err != nil || revision <= 0 {
		return ""
	}

	db := tool.DB_connect()
	defer tool.DB_close(db)

	stored_revision := ""
	if !tool.QueryRow_DB(
		db,
		"select id from history where title = ? and id + 0 = ? order by id + 0 desc limit 1",
		[]any{&stored_revision},
		doc_name,
		revision,
	) {
		return ""
	}

	commit_uuid := tool.Baro_rev_uuid(db, doc_name, stored_revision)
	if !tool.Baro_uuid_is(commit_uuid) {
		return ""
	}

	return "/history/" + tool.Url_parser(doc_name) + "?commit=" + tool.Url_parser(commit_uuid)
}

func register_history_edit_routes(r *gin.Engine) {
	r.GET("/move_all", func(c *gin.Context) {
		data := route.View_edit_move_all(make_route_config(c), nil)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})
	r.POST("/move_all", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		data := route.View_edit_move_all(make_route_config(c), c.Request.PostForm)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	})

	r.GET("/render/*path", func(c *gin.Context) {
		doc_name, revision := history_doc_revision(c, c.Param("path"))
		route_data := route.View_render(make_route_config(c), tool.Namespace_normalize(doc_name), revision)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	// 문서의 각 줄을 누가 · 언제 · 몇 판에서 썼는지 보여준다.
	// 역사 목록 · 최근 변경의 도구 상자에서 연결된다.
	r.GET("/blame/*path", func(c *gin.Context) {
		doc_name, revision := history_doc_revision(c, c.Param("path"))
		route_data := route.View_w_blame(make_route_config(c), tool.Namespace_normalize(doc_name), revision)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.GET("/history/*doc_name", func(c *gin.Context) {
		doc_name := doc_param(c)
		if target := history_revision_jump(c, doc_name); target != "" {
			c.Redirect(http.StatusFound, target)
			return
		}
		route_data := route.View_list_history(make_route_config(c), doc_name, "", "1", c.Query("commit"), c.Query("offset"))
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.POST("/history/*doc_name", func(c *gin.Context) {
		doc_name := doc_param(c)
		a := c.PostForm("a")
		b := c.PostForm("b")

		route_data := route.View_list_history_post(make_route_config(c), doc_name, a, b)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.GET("/history_page/:num/:set_type/*doc_name", func(c *gin.Context) {
		route_data := route.View_list_history(make_route_config(c), doc_param(c), c.Param("set_type"), c.Param("num"), c.Query("commit"), c.Query("offset"))
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.POST("/history_page/:num/:set_type/*doc_name", func(c *gin.Context) {
		doc_name := doc_param(c)
		a := c.PostForm("a")
		b := c.PostForm("b")

		route_data := route.View_list_history_post(make_route_config(c), doc_name, a, b)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.GET("/diff/*path", func(c *gin.Context) {
		doc_name, before_rev, after_rev := history_diff_revisions(c, c.Param("path"))
		route_data := route.View_w_diff(
			make_route_config(c),
			tool.Namespace_normalize(doc_name),
			before_rev,
			after_rev,
		)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.GET("/edit/*doc_name", func(c *gin.Context) {
		route_data := route.View_edit(make_route_config(c), doc_param(c), c.Query("load"), c.Query("section"))
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.POST("/edit/*doc_name", func(c *gin.Context) {
		doc_name := doc_param(c)
		data := c.PostForm("content")
		agree := c.PostForm("copyright_agreement")

		// AI 를 참고했다고 밝혔으면 편집 요약 뒤에 표시를 붙인다
		send := tool.Baro_ai_send(c.PostForm("send"), tool.Baro_ai_on(c.PostForm(tool.Baro_ai_field)))

		route_data := route.View_edit_post(make_route_config(c), doc_name, data, send, agree, captcha_response(c), c.PostForm("ver"), c.PostForm("section"))
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.GET("/move/*doc_name", func(c *gin.Context) {
		route_data := route.View_edit_move(make_route_config(c), doc_param(c), nil)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.POST("/move/*doc_name", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		route_data := route.View_edit_move(make_route_config(c), doc_param(c), c.Request.PostForm)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.GET("/delete/*doc_name", func(c *gin.Context) {
		doc_name := doc_param(c)
		route_data := route.View_edit_delete(make_route_config(c), doc_name)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.POST("/delete/*doc_name", func(c *gin.Context) {
		doc_name := doc_param(c)
		route_data := route.View_edit_delete_post(
			make_route_config(c),
			doc_name,
			c.PostForm("send"),
			c.PostForm("copyright_agreement"),
			captcha_response(c),
		)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.GET("/delete_file/*doc_name", func(c *gin.Context) {
		route_data := route.View_edit_file_delete(make_route_config(c), doc_param(c), nil)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.POST("/delete_file/*doc_name", func(c *gin.Context) {
		_ = c.Request.ParseForm()
		route_data := route.View_edit_file_delete(make_route_config(c), doc_param(c), c.Request.PostForm)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.GET("/revert/*path", func(c *gin.Context) {
		doc_name, revision := history_doc_revision(c, c.Param("path"))
		route_data := route.View_edit_revert(
			make_route_config(c),
			tool.Namespace_normalize(doc_name),
			revision,
		)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.POST("/revert/*path", func(c *gin.Context) {
		doc_name, revision := history_doc_revision(c, c.Param("path"))
		route_data := route.View_edit_revert_post(
			make_route_config(c),
			tool.Namespace_normalize(doc_name),
			revision,
			c.PostForm("send"),
			c.PostForm("copyright_agreement"),
			captcha_response(c),
		)
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(route_data))
	})

	r.POST("/upload", upload_post)

	r.GET("/view/*name", route.View_view_file)
	r.GET("/views/*name", route.View_view_file)
	r.HEAD("/view/*name", route.View_view_file)
	r.HEAD("/views/*name", route.View_view_file)
	r.GET("/image/*name", route.View_view_image_file)
}
