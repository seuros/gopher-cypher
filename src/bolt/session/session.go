package session

import "github.com/seuros/gopher-cypher/src/driver"

type Session interface {
	Close() error
	Run(query string, params map[string]interface{}, metaData map[string]interface{}) ([]string, []map[string]interface{}, error)
}
type session struct {
	driver driver.Driver
}

func NewSession(urlString string) (Session, error) {
	s := &session{}
	var err error
	s.driver, err = driver.NewDriver(urlString)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (d *session) Close() error {
	return d.driver.Close()
}
