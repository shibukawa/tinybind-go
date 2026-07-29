package sqlbind

import (
	"fmt"
	"net/url"
)

// A url template field maps to net/url.URL, which database/sql handles at
// neither boundary: driver.DefaultParameterConverter rejects a struct
// parameter, and Rows.Scan cannot assign a text column into one. The bind side
// is absorbed by Builder.Arg; the scan side needs an explicit target, because
// Scan receives an address and cannot be given a conversion.

// ScanURL adapts a url.URL field to Rows.Scan. Generated code passes it in
// place of the field address, so the column is parsed rather than assigned.
func ScanURL(target *url.URL) urlTarget { return urlTarget{target: target} }

// ScanOptionalURL is ScanURL for an optional field, where SQL NULL leaves a nil
// pointer instead of failing.
func ScanOptionalURL(target **url.URL) optionalURLTarget { return optionalURLTarget{target: target} }

type urlTarget struct{ target *url.URL }

func (t urlTarget) Scan(value any) error {
	text, ok, err := urlText(value)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("sqlbind: cannot scan NULL into a url field")
	}
	parsed, err := url.Parse(text)
	if err != nil {
		return fmt.Errorf("sqlbind: scan url: %w", err)
	}
	*t.target = *parsed
	return nil
}

type optionalURLTarget struct{ target **url.URL }

func (t optionalURLTarget) Scan(value any) error {
	text, ok, err := urlText(value)
	if err != nil {
		return err
	}
	if !ok {
		*t.target = nil
		return nil
	}
	parsed, err := url.Parse(text)
	if err != nil {
		return fmt.Errorf("sqlbind: scan url: %w", err)
	}
	*t.target = parsed
	return nil
}

// urlText normalizes what a driver reports for a text column. present is false
// for SQL NULL. Drivers disagree here: a PostgreSQL driver commonly yields
// string and a MySQL one yields []byte for the same column.
func urlText(value any) (text string, present bool, err error) {
	switch v := value.(type) {
	case nil:
		return "", false, nil
	case string:
		return v, true, nil
	case []byte:
		return string(v), true, nil
	}
	return "", false, fmt.Errorf("sqlbind: cannot scan %T into a url field", value)
}
