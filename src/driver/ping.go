package driver

import (
	"github.com/seuros/gopher-cypher/src/internal/boltutil"
)

func (d *driver) Ping() error {
	d.logger.Debug("Starting ping to server")
	
	conn, err := d.netPool.Get()
	defer d.netPool.Put(conn, err)
	if err != nil {
		d.logger.Error("Ping failed: unable to get connection", "error", err)
		return err
	}

	err = boltutil.CheckVersion(conn)
	if err != nil {
		d.logger.Error("Ping failed: version check", "error", err)
		return err
	}

	err = boltutil.SendHello(conn)
	if err != nil {
		d.logger.Error("Ping failed: HELLO message", "error", err)
		return err
	}

	err = boltutil.Authenticate(conn, d.urlResolver)
	if err != nil {
		d.logger.Error("Ping failed: authentication", "error", err)
		return err
	}

	d.logger.Debug("Ping successful")
	return nil
}
