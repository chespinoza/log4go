// Copyright (C) 2010, Kyle Lemons <kyle@kylelemons.net>.  All rights reserved.

package log4go

import (
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
)

type xmlProperty struct {
	Name  string `xml:"name,attr"`
	Value string `xml:",chardata"`
}

type xmlFilter struct {
	Enabled  string        `xml:"enabled,attr"`
	Tag      string        `xml:"tag"`
	Level    string        `xml:"level"`
	Type     string        `xml:"type"`
	Property []xmlProperty `xml:"property"`
}

type xmlLoggerConfig struct {
	Filter []xmlFilter `xml:"filter"`
}

// Load XML configuration; see examples/example.xml for documentation
func (log Logger) LoadConfiguration(filename string) error {

	// Open the configuration file
	fd, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("LoadConfiguration: Error: Could not open %q for reading: %s\n", filename, err)
	}
	defer fd.Close()

	// Load the configuration
	return log.LoadConfigurationFromReader(fd, filename)
}

// Load XML configuration from a reader
func (log Logger) LoadConfigurationFromReader(r io.Reader, filename string) error {
	log.Close()

	contents, err := ioutil.ReadAll(r)
	if err != nil {
		return fmt.Errorf("LoadConfiguration: Error: Could not read %q: %s\n", filename, err)
	}

	xc := new(xmlLoggerConfig)
	if err := xml.Unmarshal(contents, xc); err != nil {
		return fmt.Errorf("LoadConfiguration: Error: Could not parse XML configuration in %q: %s\n", filename, err)
	}

	for _, xmlfilt := range xc.Filter {
		var filt LogWriter
		var lvl Level
		enabled := false

		// Check required children
		if len(xmlfilt.Enabled) == 0 {
			return fmt.Errorf("LoadConfiguration: Error: Required attribute %s for filter missing in %s\n", "enabled", filename)
		} else {
			enabled = xmlfilt.Enabled != "false"
		}
		if len(xmlfilt.Tag) == 0 {
			return fmt.Errorf("LoadConfiguration: Error: Required child <%s> for filter missing in %s\n", "tag", filename)
		}
		if len(xmlfilt.Type) == 0 {
			return fmt.Errorf("LoadConfiguration: Error: Required child <%s> for filter missing in %s\n", "type", filename)
		}
		if len(xmlfilt.Level) == 0 {
			return fmt.Errorf("LoadConfiguration: Error: Required child <%s> for filter missing in %s\n", "level", filename)
		}

		switch xmlfilt.Level {
		case "FINEST":
			lvl = FINEST
		case "FINE":
			lvl = FINE
		case "DEBUG":
			lvl = DEBUG
		case "TRACE":
			lvl = TRACE
		case "INFO":
			lvl = INFO
		case "WARNING":
			lvl = WARNING
		case "ERROR":
			lvl = ERROR
		case "CRITICAL":
			lvl = CRITICAL
		default:
			return fmt.Errorf("LoadConfiguration: Error: Required child <%s> for filter has unknown value in %s: %s\n", "level", filename, xmlfilt.Level)
		}

		switch xmlfilt.Type {
		case "console":
			filt, err = xmlToConsoleLogWriter(filename, xmlfilt.Property, enabled)
		case "file":
			filt, err = xmlToFileLogWriter(filename, xmlfilt.Property, enabled)
		case "xml":
			filt, err = xmlToXMLLogWriter(filename, xmlfilt.Property, enabled)
		case "socket":
			filt, err = xmlToSocketLogWriter(filename, xmlfilt.Property, enabled)
		default:
			err = fmt.Errorf("LoadConfiguration: Error: Could not load XML configuration in %s: unknown filter type \"%s\"\n", filename, xmlfilt.Type)
		}

		// Just so all of the required params are errored at the same time if wrong
		if err != nil {
			return err
		}

		// If we're disabled (syntax and correctness checks only), don't add to logger
		if !enabled {
			continue
		}

		log[xmlfilt.Tag] = &Filter{lvl, filt}
	}

	return nil
}

func xmlToConsoleLogWriter(filename string, props []xmlProperty, enabled bool) (ConsoleLogWriter, error) {
	// Parse properties
	for _, prop := range props {
		switch prop.Name {
		default:
			fmt.Fprintf(os.Stderr, "LoadConfiguration: Warning: Unknown property \"%s\" for console filter in %s\n", prop.Name, filename)
		}
	}

	// If it's disabled, we're just checking syntax
	if !enabled {
		return nil, nil
	}

	return NewConsoleLogWriter(), nil
}

// Parse a number with K/M/G suffixes based on thousands (1000) or 2^10 (1024)
func strToNumSuffix(str string, mult int) int {
	num := 1
	if len(str) > 1 {
		switch str[len(str)-1] {
		case 'G', 'g':
			num *= mult
			fallthrough
		case 'M', 'm':
			num *= mult
			fallthrough
		case 'K', 'k':
			num *= mult
			str = str[0 : len(str)-1]
		}
	}
	parsed, _ := strconv.Atoi(str)
	return parsed * num
}
func xmlToFileLogWriter(filename string, props []xmlProperty, enabled bool) (*FileLogWriter, error) {
	file := ""
	format := "[%D %T] [%L] (%S) %M"
	maxlines := 0
	maxsize := 0
	daily := false
	rotate := false
	keepNum := 0

	// Parse properties
	for _, prop := range props {
		switch prop.Name {
		case "filename":
			file = strings.Trim(prop.Value, " \r\n")
		case "format":
			format = strings.Trim(prop.Value, " \r\n")
		case "maxlines":
			maxlines = strToNumSuffix(strings.Trim(prop.Value, " \r\n"), 1000)
		case "maxsize":
			maxsize = strToNumSuffix(strings.Trim(prop.Value, " \r\n"), 1024)
		case "daily":
			daily = strings.Trim(prop.Value, " \r\n") != "false"
		case "rotate":
			rotate = strings.Trim(prop.Value, " \r\n") != "false"
		case "keepnum":
			keepNum, _ = strconv.Atoi(strings.Trim(prop.Value, " \r\n"))
		default:
			fmt.Fprintf(os.Stderr, "LoadConfiguration: Warning: Unknown property \"%s\" for file filter in %s\n", prop.Name, filename)
		}
	}

	// Check properties
	if len(file) == 0 {
		return nil, fmt.Errorf("LoadConfiguration: Error: Required property \"%s\" for file filter missing in %s\n", "filename", filename)
	}

	// If it's disabled, we're just checking syntax
	if !enabled {
		return nil, nil
	}

	flw := NewFileLogWriter(file, rotate)
	flw.SetFormat(format)
	flw.SetRotateLines(maxlines)
	flw.SetRotateSize(maxsize)
	flw.SetRotateDaily(daily)
	flw.SetKeepNum(keepNum)
	return flw, nil
}

func xmlToXMLLogWriter(filename string, props []xmlProperty, enabled bool) (*FileLogWriter, error) {
	file := ""
	maxrecords := 0
	maxsize := 0
	daily := false
	rotate := false

	// Parse properties
	for _, prop := range props {
		switch prop.Name {
		case "filename":
			file = strings.Trim(prop.Value, " \r\n")
		case "maxrecords":
			maxrecords = strToNumSuffix(strings.Trim(prop.Value, " \r\n"), 1000)
		case "maxsize":
			maxsize = strToNumSuffix(strings.Trim(prop.Value, " \r\n"), 1024)
		case "daily":
			daily = strings.Trim(prop.Value, " \r\n") != "false"
		case "rotate":
			rotate = strings.Trim(prop.Value, " \r\n") != "false"
		default:
			fmt.Fprintf(os.Stderr, "LoadConfiguration: Warning: Unknown property \"%s\" for xml filter in %s\n", prop.Name, filename)
		}
	}

	// Check properties
	if len(file) == 0 {
		return nil, fmt.Errorf("LoadConfiguration: Error: Required property \"%s\" for xml filter missing in %s\n", "filename", filename)
	}

	// If it's disabled, we're just checking syntax
	if !enabled {
		return nil, nil
	}

	xlw := NewXMLLogWriter(file, rotate)
	xlw.SetRotateLines(maxrecords)
	xlw.SetRotateSize(maxsize)
	xlw.SetRotateDaily(daily)
	return xlw, nil
}

func xmlToSocketLogWriter(filename string, props []xmlProperty, enabled bool) (SocketLogWriter, error) {
	endpoint := ""
	protocol := "udp"

	// Parse properties
	for _, prop := range props {
		switch prop.Name {
		case "endpoint":
			endpoint = strings.Trim(prop.Value, " \r\n")
		case "protocol":
			protocol = strings.Trim(prop.Value, " \r\n")
		default:
			fmt.Fprintf(os.Stderr, "LoadConfiguration: Warning: Unknown property \"%s\" for file filter in %s\n", prop.Name, filename)
		}
	}

	// Check properties
	if len(endpoint) == 0 {
		return nil, fmt.Errorf("LoadConfiguration: Error: Required property \"%s\" for file filter missing in %s\n", "endpoint", filename)
	}

	// If it's disabled, we're just checking syntax
	if !enabled {
		return nil, nil
	}

	return NewSocketLogWriter(protocol, endpoint), nil
}
