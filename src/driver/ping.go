package driver

func (d *driver) Ping() error {
	d.logger.Debug("Starting ping to server")

	conn, err := d.netPool.Get()
	if err != nil {
		d.logger.Error("Ping failed: unable to get connection", "error", err)
		return err
	}
	defer func() {
		d.netPool.Put(conn, err)
	}()

	// Use ensureAuthenticated for consistent connection handling
	_, err = d.ensureAuthenticated(conn)
	if err != nil {
		d.logger.Error("Ping failed", "error", err)
		return err
	}

	d.logger.Debug("Ping successful")
	return nil
}
