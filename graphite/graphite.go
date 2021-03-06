package graphite

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/influxdb/influxdb"
)

const (
	// DefaultGraphitePort represents the default Graphite (Carbon) plaintext port.
	DefaultGraphitePort = 2003

	// DefaultGraphiteNameSeparator represents the default Graphite field separator.
	DefaultGraphiteNameSeparator = "."
)

var (
	// ErrBindAddressRequired is returned when starting the Server
	// without a TCP or UDP listening address.
	ErrBindAddressRequired = errors.New("bind address required")

	// ErrServerClosed return when closing an already closed graphite server.
	ErrServerClosed = errors.New("server already closed")

	// ErrDatabaseNotSpecified retuned when no database was specified in the config file
	ErrDatabaseNotSpecified = errors.New("database was not specified in config")

	// ErrServerNotSpecified returned when Server is not specified.
	ErrServerNotSpecified = errors.New("server not present")
)

// SeriesWriter defines the interface for the destination of the data.
type SeriesWriter interface {
	WriteSeries(database, retentionPolicy string, points []influxdb.Point) (uint64, error)
}

// Parser encapulates a Graphite Parser.
type Parser struct {
	Separator   string
	LastEnabled bool
}

// NewParser returns a GraphiteParser instance.
func NewParser() *Parser {
	return &Parser{Separator: DefaultGraphiteNameSeparator}
}

// Parse performs Graphite parsing of a single line.
func (p *Parser) Parse(line string) (influxdb.Point, error) {
	// Break into 3 fields (name, value, timestamp).
	fields := strings.Fields(line)
	if len(fields) != 3 {
		return influxdb.Point{}, fmt.Errorf("received %q which doesn't have three fields", line)
	}

	// decode the name and tags
	name, tags, err := p.DecodeNameAndTags(fields[0])
	if err != nil {
		return influxdb.Point{}, err
	}

	// Parse value.
	v, err := strconv.ParseFloat(fields[1], 64)
	if err != nil {
		return influxdb.Point{}, err
	}

	values := make(map[string]interface{})
	// Determine if value is a float or an int.
	if i := int64(v); float64(i) == v {
		values[name] = int64(v)
	} else {
		values[name] = v
	}

	// Parse timestamp.
	unixTime, err := strconv.ParseInt(fields[2], 10, 64)
	if err != nil {
		return influxdb.Point{}, err
	}

	timestamp := time.Unix(0, unixTime*int64(time.Millisecond))

	point := influxdb.Point{
		Name:      name,
		Tags:      tags,
		Values:    values,
		Timestamp: timestamp,
	}

	return point, nil
}

// DecodeNameAndTags parses the name and tags of a single field of a Graphite datum.
func (p *Parser) DecodeNameAndTags(field string) (string, map[string]string, error) {
	var (
		name string
		tags = make(map[string]string)
	)

	// decode the name and tags
	values := strings.Split(field, p.Separator)
	if len(values)%2 != 1 {
		// There should always be an odd number of fields to map a point name and tags
		// ex: region.us-west.hostname.server01.cpu -> tags -> region: us-west, hostname: server01, point name -> cpu
		return name, tags, fmt.Errorf("received %q which doesn't conform to format of key.value.key.value.name or name", field)
	}

	if p.LastEnabled {
		name = values[len(values)-1]
		values = values[0 : len(values)-1]
	} else {
		name = values[0]
		values = values[1:]
	}

	if name == "" {
		return name, tags, fmt.Errorf("no name specified for metric. %q", field)
	}

	// Grab the pairs and throw them in the map
	for i := 0; i < len(values); i += 2 {
		k := values[i]
		v := values[i+1]
		tags[k] = v
	}

	return name, tags, nil
}
