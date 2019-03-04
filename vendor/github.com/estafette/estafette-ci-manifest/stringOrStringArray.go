package manifest

import "encoding/json"

// StringOrStringArray is used to unmarshal/marshal either a single string value or a string array
type StringOrStringArray struct {
	Values []string
}

// UnmarshalYAML customizes unmarshalling a StringOrStringArray
func (s *StringOrStringArray) UnmarshalYAML(unmarshal func(interface{}) error) (err error) {
	var multi []string
	err = unmarshal(&multi)
	if err != nil {
		var single string
		err := unmarshal(&single)
		if err != nil {
			return err
		}
		if single != "" {
			s.Values = make([]string, 1)
			s.Values[0] = single
		}
	} else {
		s.Values = multi
	}
	return nil
}

// MarshalYAML customizes marshalling a StringOrStringArray
func (s StringOrStringArray) MarshalYAML() (out interface{}, err error) {

	if len(s.Values) == 1 {
		return s.Values[0], err
	} else if len(s.Values) > 0 {
		return s.Values, err
	}

	return "", err
}

// UnmarshalJSON customizes unmarshalling a StringOrStringArray
func (s *StringOrStringArray) UnmarshalJSON(b []byte) error {

	var multi []string
	err := json.Unmarshal(b, &multi)
	if err != nil {
		var single string
		err := json.Unmarshal(b, &single)
		if err != nil {
			return err
		}
		if single != "" {
			s.Values = make([]string, 1)
			s.Values[0] = single
		}
	} else {
		s.Values = multi
	}
	return nil
}

// MarshalJSON customizes marshalling a StringOrStringArray
func (s StringOrStringArray) MarshalJSON() ([]byte, error) {

	if len(s.Values) == 1 {
		return json.Marshal(s.Values[0])
	} else if len(s.Values) > 0 {
		return json.Marshal(s.Values)
	}

	return json.Marshal("")
}

// Contains test whether the compared value matches one of the values
func (s StringOrStringArray) Contains(value string) bool {
	for _, v := range s.Values {
		if v == value {
			return true
		}
	}
	return false
}
