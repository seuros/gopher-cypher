package session

import "context"

func (s *session) Run(ctx context.Context, query string, params map[string]interface{}, metaData map[string]interface{}) ([]string, []map[string]interface{}, error) {
	cols, rows, err := s.driver.Run(ctx, query, params, metaData)
	if err != nil {
		return nil, nil, err
	}
	return cols, rows, nil
}
