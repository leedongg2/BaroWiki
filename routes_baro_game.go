package main

import (
	"net/http"

	"opennamu/route"

	"github.com/gin-gonic/gin"
)

// baro_game_form : 폼 값을 이름 → 값 하나로 (놀이 화면은 값이 몇 개뿐이다)
func baro_game_form(c *gin.Context) map[string]string {
	_ = c.Request.ParseForm()

	out := map[string]string{}
	for name, value := range c.Request.PostForm {
		if len(value) > 0 {
			out[name] = value[0]
		}
	}

	return out
}

// register_baro_game_routes : 바로게임 라우트
func register_baro_game_routes(r *gin.Engine) {
	html := func(c *gin.Context, data string) {
		write_data(c, http.StatusOK, "text/html; charset=utf-8", []byte(data))
	}

	r.GET("/baro/game", func(c *gin.Context) {
		html(c, route.View_baro_game(make_route_config(c)))
	})

	r.GET("/baro/game/quiz", func(c *gin.Context) {
		html(c, route.View_baro_game_quiz(make_route_config(c), nil))
	})

	r.POST("/baro/game/quiz", func(c *gin.Context) {
		html(c, route.View_baro_game_quiz(make_route_config(c), baro_game_form(c)))
	})

	r.GET("/baro/game/race", func(c *gin.Context) {
		html(c, route.View_baro_game_race(make_route_config(c), nil))
	})

	r.POST("/baro/game/race", func(c *gin.Context) {
		html(c, route.View_baro_game_race(make_route_config(c), baro_game_form(c)))
	})

	r.GET("/baro/game/rank", func(c *gin.Context) {
		html(c, route.View_baro_game_rank(make_route_config(c)))
	})
}
