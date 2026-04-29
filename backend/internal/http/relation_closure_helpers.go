package http

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

func parseAgentIDParam(c *gin.Context) (uint64, error) {
	id, err := strconv.ParseUint(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id == 0 {
		return 0, errors.New("invalid agent id")
	}
	return id, nil
}

func parseHierarchyDepthRange(c *gin.Context) (hierarchyDepthRange, error) {
	depthRange := hierarchyDepthRange{Min: 1}
	if minDepthText := strings.TrimSpace(c.Query("minDepth")); minDepthText != "" {
		minDepth, err := strconv.ParseUint(minDepthText, 10, 32)
		if err != nil {
			return depthRange, errors.New("minDepth must be a non-negative integer")
		}
		if minDepth == 0 {
			return depthRange, errors.New("minDepth must be greater than zero")
		}
		depthRange.Min = uint32(minDepth)
	}
	if maxDepthText := strings.TrimSpace(c.Query("maxDepth")); maxDepthText != "" {
		maxDepth, err := strconv.ParseUint(maxDepthText, 10, 32)
		if err != nil {
			return depthRange, errors.New("maxDepth must be a non-negative integer")
		}
		if maxDepth == 0 {
			return depthRange, errors.New("maxDepth must be greater than zero")
		}
		maxDepthValue := uint32(maxDepth)
		depthRange.Max = &maxDepthValue
	}
	if depthRange.Max != nil && depthRange.Min > *depthRange.Max {
		return depthRange, errors.New("minDepth cannot be greater than maxDepth")
	}
	return depthRange, nil
}
