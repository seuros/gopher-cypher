package session

func (s *session) Run(query string, params map[string]interface{}, metaData map[string]interface{}) ([]string, []map[string]interface{}, error) {
	cols, rows, err := s.driver.Run(query, params, metaData)
	if err != nil {
		return nil, nil, err
	}
	return cols, rows, nil
}
