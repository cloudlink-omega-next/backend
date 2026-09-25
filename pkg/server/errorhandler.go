package server

import (
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func (s *Server) ErrorPage(c *fiber.Ctx, err error) error {
	fmt.Println("[DEBUG] ErrorPage called with error:", err.Error())
	var status_code int
	if err == nil {
		status_code = fiber.StatusInternalServerError
	} else {
		switch e := err.(type) {
		case *fiber.Error:
			status_code = e.Code
		default:
			status_code = fiber.StatusInternalServerError
		}
	}

	c.Status(status_code)

	request_content_type := string(c.Request().Header.ContentType())

	if strings.Contains(request_content_type, "json") {
		return c.JSON(fiber.Map{
			"result": err.Error(),
			"data":   nil,
		})
	}

	return c.Render("views/error", &map[string]string{
		"Message":    err.Error(),
		"Status":     fmt.Sprint(status_code),
		"ServerName": s.ServerName}, "views/layouts/error")
}
