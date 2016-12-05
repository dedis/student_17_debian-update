package services

import (
	// Importing the services so they register their services to SDA
	// automatically when importing github.com/dedis/cothority/services
	_ "github.com/dedis/cosi/service"
	_ "github.com/dedis/cothority/services/debianupdate"
	_ "github.com/dedis/cothority/services/guard"
	_ "github.com/dedis/cothority/services/identity"
	_ "github.com/dedis/cothority/services/skipchain"
	_ "github.com/dedis/cothority/services/status"
	_ "github.com/dedis/cothority/services/swupdate"
)
