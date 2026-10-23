package api

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

// dockerStatus reports daemon availability.
func (s *Server) dockerStatus(c *gin.Context) {
	ok(c, s.Docker.Status(c.Request.Context()))
}

// dockerContainers lists containers with live stats when requested.
func (s *Server) dockerContainers(c *gin.Context) {
	containers, err := s.Docker.ListContainers(c.Request.Context(), c.Query("all") != "0")
	if err != nil {
		apiError(c, http.StatusBadGateway, "docker_unavailable", "无法连接 Docker: "+err.Error())
		return
	}
	if c.Query("stats") == "1" {
		for i := range containers {
			if containers[i].State != "running" {
				continue
			}
			if st, err := s.Docker.Stats(c.Request.Context(), containers[i].ID); err == nil {
				containers[i].CPU = st.CPU
				containers[i].Memory = st.MemUsage
			}
		}
	}
	ok(c, containers)
}

// dockerAction starts/stops/restarts a container.
func (s *Server) dockerAction(c *gin.Context) {
	id := c.Param("id")
	action := c.Param("action")
	if err := s.Docker.Action(c.Request.Context(), id, action); err != nil {
		apiError(c, http.StatusBadGateway, "docker_error", err.Error())
		return
	}
	s.audit(c, "docker", "container_"+action, id, "容器操作", "ok")
	ok(c, gin.H{"message": "操作已执行"})
}

// dockerRemove deletes a container.
func (s *Server) dockerRemove(c *gin.Context) {
	if err := s.Docker.Remove(c.Request.Context(), c.Param("id"), c.Query("force") == "1"); err != nil {
		apiError(c, http.StatusBadGateway, "docker_error", err.Error())
		return
	}
	s.audit(c, "docker", "container_remove", c.Param("id"), "删除容器", "ok")
	ok(c, gin.H{"message": "容器已删除"})
}

// dockerLogs returns container log output.
func (s *Server) dockerLogs(c *gin.Context) {
	logs, err := s.Docker.Logs(c.Request.Context(), c.Param("id"), intParam(c, "tail", 300))
	if err != nil {
		apiError(c, http.StatusBadGateway, "docker_error", err.Error())
		return
	}
	ok(c, gin.H{"logs": logs})
}

// dockerStats returns one container stats sample.
func (s *Server) dockerStats(c *gin.Context) {
	st, err := s.Docker.Stats(c.Request.Context(), c.Param("id"))
	if err != nil {
		apiError(c, http.StatusBadGateway, "docker_error", err.Error())
		return
	}
	ok(c, st)
}

// dockerImages lists images.
func (s *Server) dockerImages(c *gin.Context) {
	images, err := s.Docker.ListImages(c.Request.Context())
	if err != nil {
		apiError(c, http.StatusBadGateway, "docker_error", err.Error())
		return
	}
	ok(c, images)
}

// dockerPull pulls an image as a background job.
func (s *Server) dockerPull(c *gin.Context) {
	var req struct {
		Image string `json:"image" binding:"required"`
	}
	if !bindJSON(c, &req) {
		return
	}
	job := s.App.Jobs.Start("拉取镜像 "+req.Image, func(ctx context.Context, report func(int, string)) error {
		return s.Docker.PullImage(ctx, req.Image, report)
	})
	s.audit(c, "docker", "image_pull", req.Image, "拉取镜像", "ok")
	created(c, job)
}

// dockerRemoveImage deletes an image.
func (s *Server) dockerRemoveImage(c *gin.Context) {
	if err := s.Docker.RemoveImage(c.Request.Context(), c.Param("id")); err != nil {
		apiError(c, http.StatusBadGateway, "docker_error", err.Error())
		return
	}
	s.audit(c, "docker", "image_remove", c.Param("id"), "删除镜像", "ok")
	ok(c, gin.H{"message": "镜像已删除"})
}

// composeList lists compose projects.
func (s *Server) composeList(c *gin.Context) {
	projects, err := s.Docker.ListCompose()
	if err != nil {
		apiError(c, http.StatusInternalServerError, "compose_error", err.Error())
		return
	}
	ok(c, projects)
}

// composeCreate stores a compose file.
func (s *Server) composeCreate(c *gin.Context) {
	var req struct {
		Name    string `json:"name" binding:"required"`
		Content string `json:"content" binding:"required"`
	}
	if !bindJSON(c, &req) {
		return
	}
	project, err := s.Docker.CreateCompose(req.Name, req.Content)
	if err != nil {
		apiError(c, http.StatusBadRequest, "compose_error", err.Error())
		return
	}
	s.audit(c, "docker", "compose_create", req.Name, "创建 compose 项目", "ok")
	created(c, project)
}

// composeDelete removes a project.
func (s *Server) composeDelete(c *gin.Context) {
	name := c.Param("name")
	if err := s.Docker.DeleteCompose(c.Request.Context(), name); err != nil {
		apiError(c, http.StatusBadRequest, "compose_error", err.Error())
		return
	}
	s.audit(c, "docker", "compose_delete", name, "删除 compose 项目", "ok")
	ok(c, gin.H{"message": "项目已删除"})
}

// composeUp deploys a project.
func (s *Server) composeUp(c *gin.Context) {
	name := c.Param("name")
	out, err := s.Docker.ComposeUp(c.Request.Context(), name)
	if err != nil {
		apiError(c, http.StatusBadGateway, "compose_error", err.Error())
		return
	}
	s.audit(c, "docker", "compose_up", name, "部署 compose 项目", "ok")
	ok(c, gin.H{"message": "部署完成", "output": out})
}

// composeDown stops a project.
func (s *Server) composeDown(c *gin.Context) {
	name := c.Param("name")
	out, err := s.Docker.ComposeDown(c.Request.Context(), name)
	if err != nil {
		apiError(c, http.StatusBadGateway, "compose_error", err.Error())
		return
	}
	s.audit(c, "docker", "compose_down", name, "停止 compose 项目", "ok")
	ok(c, gin.H{"message": "已停止", "output": out})
}

// composeLogs returns project logs.
func (s *Server) composeLogs(c *gin.Context) {
	logs, err := s.Docker.ComposeLogs(c.Request.Context(), c.Param("name"), intParam(c, "tail", 200))
	if err != nil {
		apiError(c, http.StatusBadGateway, "compose_error", err.Error())
		return
	}
	ok(c, gin.H{"logs": logs})
}
