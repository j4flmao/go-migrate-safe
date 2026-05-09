package internal

import "log"

// StudioRun is a placeholder for MongoDB. The GMS Studio UI currently
// only supports SQL providers (postgres, mysql, sqlite, mssql).
func StudioRun(uri, addr string, openBrowser bool) {
	log.Print("gms studio: MongoDB is not yet supported by GMS Studio.")
	log.Print("            Studio currently supports postgres, mysql, sqlite, and mssql.")
}
