package main

import (
	"encoding/json"
	"os"
	"time"
)

type state struct {
	path     string
	Updaters map[string]updaterState
}

type updaterState struct {
	DataDir        string
	URI            string
	Username       string
	Password       string
	UpdateInterval time.Duration
	Variables      map[string]string
}

func readState(path string) (*state, error) {
	s := &state{
		path:     path,
		Updaters: make(map[string]updaterState),
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			err = s.save()
			if err != nil {
				return nil, err
			}
			return s, nil
		}
		return nil, err
	}
	defer f.Close()
	err = json.NewDecoder(f).Decode(s)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (s *state) save() error {
	f, err := os.Create(s.path)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(s)
}
