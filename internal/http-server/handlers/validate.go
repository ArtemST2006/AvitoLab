package handlers

import (
	"errors"
	"fmt"

	"github.com/google/uuid"

	api "github.com/ArtemST2006/AvitoLab/internal/generated"
)

const (
	minLatitude  = -90.0
	maxLatitude  = 90.0
	minLongitude = -180.0
	maxLongitude = 180.0
)

func validateTripData(d api.TripData) error {
	if d.UserId == uuid.Nil {
		return errors.New("user_id is required")
	}
	if d.DriverId == uuid.Nil {
		return errors.New("driver_id is required")
	}
	if err := validateCoordinates("start_point", d.StartPoint); err != nil {
		return err
	}
	if err := validateCoordinates("end_point", d.EndPoint); err != nil {
		return err
	}
	if d.Price < 0 {
		return errors.New("price must be >= 0")
	}
	return nil
}

func validateCoordinates(point string, p api.Coordinates) error {
	if p.Latitude < minLatitude || p.Latitude > maxLatitude {
		return fmt.Errorf("%s: latitude %.6f must be between %.0f and %.0f", point, p.Latitude, minLatitude, maxLatitude)
	}
	if p.Longitude < minLongitude || p.Longitude > maxLongitude {
		return fmt.Errorf("%s: longitude %.6f must be between %.0f and %.0f", point, p.Longitude, minLongitude, maxLongitude)
	}
	return nil
}
