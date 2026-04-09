package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Mock interface for LocationService as it wasn't provided in full
type LocationService interface {
	GetLocationByIDAsString(locationID, sourceSystem, sourceChannel string, store int) (string, error)
	GetLocationByID(locationID string) (string, error)
}

type LocationController struct {
	service LocationService
}

func NewLocationController(service LocationService) *LocationController {
	return &LocationController{service: service}
}

func (lc *LocationController) RegisterRoutes(router *gin.Engine) {
	router.GET("/locations/:locationId/booleanattributestest", lc.getLocation)
	router.GET("/locations/:locationId/booleantest", lc.getLocationBooleanTest)
}

func (lc *LocationController) getLocation(c *gin.Context) {
	locationID := c.Param("locationId")
	sourceSystem := c.Query("sourceSystem")
	sourceChannel := c.Query("sourceChannel")

	// Convert query param to int, default 0 if missing/error for simplicity here
	store := 0

	result, err := lc.service.GetLocationByIDAsString(locationID, sourceSystem, sourceChannel, store)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.String(http.StatusOK, result)
}

func (lc *LocationController) getLocationBooleanTest(c *gin.Context) {
	locationID := c.Param("locationId")
	// store, sourceSystem, sourceChannel are ignored in the original Java method body for this endpoint

	result, err := lc.service.GetLocationByID(locationID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.String(http.StatusOK, result)
}
