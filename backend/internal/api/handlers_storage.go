package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"privacyos/backend/internal/storage"
)

// storageDisks lists physical disks and SMART health.
func (s *Server) storageDisks(c *gin.Context) {
	disks, err := s.Storage.ListDisks(c.Request.Context())
	if err != nil {
		apiError(c, http.StatusInternalServerError, "storage_error", "读取磁盘失败: "+err.Error())
		return
	}
	ok(c, disks)
}

// storageSmart returns raw SMART data.
func (s *Server) storageSmart(c *gin.Context) {
	device := c.Query("device")
	if device == "" {
		apiError(c, http.StatusBadRequest, "missing_device", "缺少 device 参数")
		return
	}
	if !s.requireAdmin(c) {
		return
	}
	doc, err := s.Storage.Smart(c.Request.Context(), device)
	if err != nil {
		apiError(c, http.StatusBadGateway, "smart_error", err.Error())
		return
	}
	ok(c, doc)
}

// storageSmartTest runs a SMART self-test.
func (s *Server) storageSmartTest(c *gin.Context) {
	if !s.requireAdmin(c) {
		return
	}
	var req struct {
		Device string `json:"device" binding:"required"`
		Kind   string `json:"kind"`
	}
	if !bindJSON(c, &req) {
		return
	}
	test, err := s.Storage.RunSmartTest(c.Request.Context(), req.Device, req.Kind)
	if err != nil {
		apiError(c, http.StatusBadGateway, "smart_error", err.Error())
		return
	}
	s.audit(c, "storage", "smart_test", req.Device, "执行 SMART "+req.Kind+" 测试", "ok")
	created(c, test)
}

// storageSmartTests lists stored SMART tests.
func (s *Server) storageSmartTests(c *gin.Context) {
	tests, err := s.Store.ListSmartTests(c.Query("device"), 20)
	if err != nil {
		apiError(c, http.StatusInternalServerError, "query_failed", "读取测试记录失败")
		return
	}
	ok(c, tests)
}

// storagePools lists RAID pools.
func (s *Server) storagePools(c *gin.Context) {
	pools, err := s.Storage.ListPools()
	if err != nil {
		apiError(c, http.StatusInternalServerError, "storage_error", "读取存储池失败")
		return
	}
	ok(c, pools)
}

// storagePoolCreate creates a RAID pool.
func (s *Server) storagePoolCreate(c *gin.Context) {
	if !s.requireAdmin(c) {
		return
	}
	var req struct {
		Name       string   `json:"name" binding:"required"`
		Level      string   `json:"level" binding:"required"`
		FS         string   `json:"fs"`
		Devices    []string `json:"devices" binding:"required"`
		Mountpoint string   `json:"mountpoint"`
		Confirm    string   `json:"confirm"`
	}
	if !bindJSON(c, &req) {
		return
	}
	if req.Confirm != "CREATE" {
		apiError(c, http.StatusBadRequest, "confirm_required", "危险操作需要确认：请在 confirm 字段填写 CREATE")
		return
	}
	validLevels := map[string]bool{"raid0": true, "raid1": true, "raid5": true, "raid6": true, "raid10": true, "linear": true}
	if !validLevels[req.Level] {
		apiError(c, http.StatusBadRequest, "invalid_level", "不支持的 RAID 级别")
		return
	}
	pool, err := s.Storage.CreatePool(c.Request.Context(), req.Name, req.Level, req.FS, req.Devices, req.Mountpoint)
	if err != nil {
		apiError(c, http.StatusBadRequest, "storage_error", "创建存储池失败: "+err.Error())
		return
	}
	s.audit(c, "storage", "pool_create", req.Name, fmt.Sprintf("创建 %s 存储池", req.Level), "ok")
	created(c, pool)
}

// storagePoolMount mounts a pool.
func (s *Server) storagePoolMount(c *gin.Context) {
	if !s.requireAdmin(c) {
		return
	}
	if err := s.Storage.MountPool(c.Request.Context(), c.Param("id")); err != nil {
		apiError(c, http.StatusBadRequest, "storage_error", err.Error())
		return
	}
	s.audit(c, "storage", "pool_mount", c.Param("id"), "挂载存储池", "ok")
	ok(c, gin.H{"message": "已挂载"})
}

// storagePoolUnmount unmounts a pool.
func (s *Server) storagePoolUnmount(c *gin.Context) {
	if !s.requireAdmin(c) {
		return
	}
	if err := s.Storage.UnmountPool(c.Request.Context(), c.Param("id")); err != nil {
		apiError(c, http.StatusBadRequest, "storage_error", err.Error())
		return
	}
	s.audit(c, "storage", "pool_unmount", c.Param("id"), "卸载存储池", "ok")
	ok(c, gin.H{"message": "已卸载"})
}

// storagePoolDelete destroys a pool.
func (s *Server) storagePoolDelete(c *gin.Context) {
	if !s.requireAdmin(c) {
		return
	}
	var req struct {
		Confirm string `json:"confirm"`
		Wipe    bool   `json:"wipe"`
	}
	_ = c.ShouldBindJSON(&req)
	if req.Confirm != "DELETE" {
		apiError(c, http.StatusBadRequest, "confirm_required", "删除存储池需要确认：请在 confirm 字段填写 DELETE")
		return
	}
	if err := s.Storage.DeletePool(c.Request.Context(), c.Param("id"), req.Wipe); err != nil {
		apiError(c, http.StatusBadRequest, "storage_error", err.Error())
		return
	}
	s.audit(c, "storage", "pool_delete", c.Param("id"), "删除存储池", "ok")
	ok(c, gin.H{"message": "存储池已删除"})
}

// storagePoolExtend adds disks to a pool.
func (s *Server) storagePoolExtend(c *gin.Context) {
	if !s.requireAdmin(c) {
		return
	}
	var req struct {
		Devices []string `json:"devices" binding:"required"`
	}
	if !bindJSON(c, &req) {
		return
	}
	if err := s.Storage.ExtendPool(c.Request.Context(), c.Param("id"), req.Devices); err != nil {
		apiError(c, http.StatusBadRequest, "storage_error", err.Error())
		return
	}
	s.audit(c, "storage", "pool_extend", c.Param("id"), "扩容存储池", "ok")
	ok(c, gin.H{"message": "扩容操作已执行"})
}

// storageMounts lists mounted filesystems.
func (s *Server) storageMounts(c *gin.Context) {
	mounts, err := s.Storage.Mounts(c.Request.Context())
	if err != nil {
		apiError(c, http.StatusInternalServerError, "storage_error", err.Error())
		return
	}
	ok(c, mounts)
}

// storageUsage returns disk usage for a path.
func (s *Server) storageUsage(c *gin.Context) {
	path := c.DefaultQuery("path", "/data")
	used, pct, err := storage.Usage(path)
	if err != nil {
		apiError(c, http.StatusBadRequest, "usage_error", err.Error())
		return
	}
	ok(c, gin.H{"path": path, "used": used, "percent": pct})
}
