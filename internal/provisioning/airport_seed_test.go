package provisioning

import (
	"strings"
	"testing"
)

func TestParseAirportSeedCSV(t *testing.T) {
	t.Parallel()

	csvData := `iata_code,icao_code,name,city,country,timezone,latitude,longitude,terminals,transit_info
JFK,KJFK,John F Kennedy International Airport,New York,US,America/New_York,40.6413,-73.7781,1|4|5|7|8,subway|airtrain|bus
LAX,KLAX,Los Angeles International Airport,Los Angeles,US,America/Los_Angeles,33.9416,-118.4085,T1|T2|T3,bus|rideshare|rental_car`
	records, err := ParseAirportSeedCSV(strings.NewReader(csvData))
	if err != nil {
		t.Fatalf("parse airport csv: %v", err)
	}
	if len(records) != 2 || records[0].IATACode != "JFK" || records[1].ICAOCode != "KLAX" {
		t.Fatalf("unexpected parsed airport records: %+v", records)
	}
}
