package controller

import (
	"net/http"

	"yourmodule/service"

	"github.com/gin-gonic/gin"
	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
)

type LocationController struct {
	webClientService *service.WebClientService
}

func NewLocationController(wcs *service.WebClientService) *LocationController {
	return &LocationController{webClientService: wcs}
}

func (lc *LocationController) RegisterRoutes(router *gin.Engine) {
	router.GET("/locations/:locationId/booleanattributestest", lc.GetLocation)
	router.GET("/locations/:locationId/booleantest", lc.GetLocationBooleanTest)
}

func (lc *LocationController) GetLocation(c *gin.Context) {
	locationId := c.Param("locationId")
	sourceSystem := c.Query("sourceSystem")
	sourceChannel := c.Query("sourceChannel")
	store := c.Query("store")

	// Construct LaunchDarkly Context (Example)
	ldCtx := ldcontext.NewBuilder(locationId).
		SetString("sourceSystem", sourceSystem).
		SetString("sourceChannel", sourceChannel).
		SetString("store", store).
		Build()

	// Call generic web service - mimicking the LocationService delegation
	resp, err := lc.webClientService.Get("locationService", ldCtx, "location-feature-flag", locationId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.Data(http.StatusOK, "application/json; charset=utf-8", []byte(resp))
}

func (lc *LocationController) GetLocationBooleanTest(c *gin.Context) {
	locationId := c.Param("locationId")

	// Empty/Anonymous context if none provided
	ldCtx := ldcontext.NewBuilder(locationId).Build()

	resp, err := lc.webClientService.Get("locationService", ldCtx, "location-feature-flag", locationId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.Data(http.StatusOK, "application/json; charset=utf-8", []byte(resp))
}
