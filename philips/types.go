package philips

import (
	"fmt"
	"strconv"
)

// Info is the object returned by /sys/dev/sync
type Info struct {
	DeviceID  string `json:"device_id"`
	ModelID   string `json:"model_id"`
	Name      string `json:"name"`
	Option    string `json:"option"`
	ProductID string `json:"product_id"`
	SWVersion string `json:"swversion"`
	Type      string `json:"type"`
}

// AirQuality is the indoor air quality
type AirQuality int

// ToHemtjanst converts values as reported by Philips to their equivalent
// HomeKit stringified counterpart
func (a AirQuality) ToHemtjanst() string {
	switch a {
	case 1:
		return "1"
	case 2, 3:
		return "2"
	case 4, 5, 6:
		return "3"
	case 7, 8, 9:
		return "4"
	case 10, 11, 12:
		return "5"
	default:
		return "5"
	}
}

// Brightness level of the display/ring
type Brightness int

// ToHemtjanst converts values as reported by Philips to their equivalent
// HomeKit stringified counterpart
func (b Brightness) ToHemtjanst() string {
	return strconv.Itoa(int(b))
}

// DisplayMode represents which value is shown on the display
type DisplayMode string

// ErrorCode defines specific errors the machine can report
type ErrorCode int

func (e ErrorCode) String() string {
	switch e {
	case ErrCleanFilter:
		return fmt.Sprintf("Error: %d, one of the filters/wick needs cleaning", e)
	case ErrNoWater:
		return fmt.Sprintf("Error: %d, refill water tank", e)
	case ErrWaterTankOpen:
		return fmt.Sprintf("Error: %d, water tank is open", e)
	default:
		return fmt.Sprintf("Error: %d, unknown", e)
	}
}

// FanSpeed is the speed at which the fan functions
type FanSpeed string

// ToHemtjanst converts values as reported by Philips to their equivalent
// HomeKit stringified counterpart
func (f FanSpeed) ToHemtjanst() string {
	switch f {
	case Silent:
		return "5"
	case Speed1:
		return "20"
	case Speed2:
		return "40"
	case Speed3:
		return "80"
	case Turbo:
		return "100"
	default:
		return "0"
	}
}

// Function is either purification or purification and humidification
type Function string

// ToHemtjanst converts values as reported by Philips to their equivalent
// HomeKit stringified counterpart
func (f Function) ToHemtjanst() string {
	switch f {
	case PurificationHumidification:
		return "2"
	default:
		return "0"
	}
}

// Mode is the device operating mode
type Mode string

// Power indicates whether the device is on or off
type Power string

// ToHemtjanst converts values as reported by Philips to their equivalent
// HomeKit stringified counterpart
func (p Power) ToHemtjanst() string {
	return string(p)
}

const (
	// Silent is the lowest fan speed
	Silent FanSpeed = "s"
	// Speed1 is the lower fan speed
	Speed1 FanSpeed = "1"
	// Speed2 is the medium fan speed
	Speed2 FanSpeed = "2"
	// Speed3 is the higher fan speed
	Speed3 FanSpeed = "3"
	// Turbo is the highest fan speed
	Turbo FanSpeed = "t"

	// Off indicates the device is turned off
	Off Power = "0"
	// On indicates the device is on
	On Power = "1"

	// Brightness0 is the lowest brightness, essentially off
	Brightness0 Brightness = 0
	// Brightness25 is 25% brightness
	Brightness25 Brightness = 25
	// Brightness50 is 50% brightness
	Brightness50 Brightness = 50
	// Brightness75 is 75% brightness
	Brightness75 Brightness = 75
	// Brightness100 is the max brightness, essentially on
	Brightness100 Brightness = 100

	// Auto is the automatic mode
	Auto Mode = "P"
	// Allergen is the allergen filtering
	Allergen Mode = "A"
	// Sleep is the quitest mode
	Sleep Mode = "S"
	// Manual allows full control
	Manual Mode = "M"
	// Bacteria filters for bacteria
	Bacteria Mode = "B"
	// Night mode is for use in bedrooms at night
	Night Mode = "N"

	// Purification indicates it's only purifiying
	Purification Function = "P"
	// PurificationHumidification does both
	PurificationHumidification Function = "PH"

	// IAQ or Indoor air quality is shown on the display
	IAQ DisplayMode = "0"
	// PM25 shows the particulate matter 2.5 value on the display
	PM25 DisplayMode = "1"
	// Humidity shows the current humidity on the display
	Humidity DisplayMode = "3"

	// ErrNoWater indicates the water tank is empty
	ErrNoWater ErrorCode = 49408
	// ErrWaterTankOpen indicates the water tank is left open
	ErrWaterTankOpen ErrorCode = 32768
	// ErrCleanFilter indicates it's time to clean a filter
	ErrCleanFilter ErrorCode = 49155
)

// Status is the status object returned by the /sys/dev/status endpoint
type Status struct {
	State struct {
		Reported struct {
			// Device name, as it shows in the app
			Name string `json:"name"`
			// Device model
			Type string `json:"type"`
			// Device model ID, same as Type but with /XX at the end
			ModelID         string `json:"modelid"`
			FirmwareVersion string `json:"swversion"`
			// Is 0.0.0 always?
			DeviceVersion string `json:"DeviceVersion"`
			// Over The Air update
			// No idea what the value means, seeing 'ck' on one
			OTA string `json:"ota"`
			// Amount of hours the device has been powered on
			Runtime     int    `json:"Runtime"`
			WiFiVersion string `json:"WifiVersion"`
			// Some crazy long identifier
			ProductID string `json:"ProductId"`
			// Some crazy long identifier
			DeviceID string `json:"DeviceId"`
			// How status is reported, 'localcontrol'
			StatusType string `json:"StatusType"`
			// If it's connected to cloud, 'Localcontrol'
			ConnectType string   `json:"ConnectType"`
			FanSpeed    FanSpeed `json:"om"`
			// Is the device powered on, 1 yes, 0 no
			Power Power `json:"pwr"`
			// Is child lock enabled, i.e do the buttons on the device respond
			ChildLock bool `json:"cl"`
			// Brightness of the display/ring
			Brightness Brightness `json:"aqil"`
			// Backlight of the buttons
			ButtonBacklight string `json:"uil"`
			// Hours set on the timer
			Timer int `json:"dt"`
			// Time left on the timer in minutes
			TimerTimeLeft int `json:"dtrs"`
			// Operating mode of the device
			Mode Mode `json:"mode"`
			// Activated function
			Function Function `json:"func"`
			// Desired relative humidity
			RelativeHumidityTarget int `json:"rhset"`
			// Meassured relative humidity
			RelativeHumidity    int        `json:"rh"`
			Temperature         int        `json:"temp"`
			ParticulateMatter25 int        `json:"pm25"`
			AirQuality          AirQuality `json:"iaql"`
			// App push notification when air quality crosses a threshold
			AirQuailityIndexNotificationThreshold int `json:"aqit"`
			// What value is shown on the display
			DisplayMode DisplayMode `json:"ddp"`
			// Unknown, seemingly always 0?
			Rddp string `json:"rddp"`
			// Error code
			Err        ErrorCode `json:"err"`
			WaterLevel int       `json:"wl"`
			// Which code gets displayed when a filter needs replacement
			HEPAFilterReplacementCode         string `json:"fltt1"`
			ActiveCarbonFilterReplacementCode string `json:"fltt2"`
			PrefilterAndWickCleanIn           int    `json:"fltsts0"`
			HEPAFilterReplaceIn               int    `json:"fltsts1"`
			ActiveCarbonFilterReplaceIn       int    `json:"fltsts2"`
			WickReplaceIn                     int    `json:"wicksts"`
		} `json:"reported"`
	} `json:"state"`
}
