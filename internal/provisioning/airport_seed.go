package provisioning

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type AirportRecord struct {
	IATACode    string
	ICAOCode    string
	Name        string
	City        string
	Country     string
	Timezone    string
	Latitude    float64
	Longitude   float64
	Terminals   string
	TransitInfo string
}

func ParseAirportSeedCSV(reader io.Reader) ([]AirportRecord, error) {
	csvReader := csv.NewReader(reader)
	csvReader.TrimLeadingSpace = true
	rows, err := csvReader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) <= 1 {
		return []AirportRecord{}, nil
	}
	records := make([]AirportRecord, 0, len(rows)-1)
	for i := 1; i < len(rows); i++ {
		row := rows[i]
		if len(row) < 10 {
			return nil, fmt.Errorf("airport csv row %d has insufficient columns", i+1)
		}
		lat, err := strconv.ParseFloat(strings.TrimSpace(row[6]), 64)
		if err != nil {
			return nil, fmt.Errorf("airport csv row %d invalid latitude: %w", i+1, err)
		}
		lon, err := strconv.ParseFloat(strings.TrimSpace(row[7]), 64)
		if err != nil {
			return nil, fmt.Errorf("airport csv row %d invalid longitude: %w", i+1, err)
		}
		record := AirportRecord{
			IATACode:    strings.ToUpper(strings.TrimSpace(row[0])),
			ICAOCode:    strings.ToUpper(strings.TrimSpace(row[1])),
			Name:        strings.TrimSpace(row[2]),
			City:        strings.TrimSpace(row[3]),
			Country:     strings.ToUpper(strings.TrimSpace(row[4])),
			Timezone:    strings.TrimSpace(row[5]),
			Latitude:    lat,
			Longitude:   lon,
			Terminals:   strings.TrimSpace(row[8]),
			TransitInfo: strings.TrimSpace(row[9]),
		}
		records = append(records, record)
	}
	return records, nil
}
