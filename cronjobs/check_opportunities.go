package cronjobs

import (
	"c_trd/services"
	"log"
	"time"
)

// ProcessExpiredOpportunities cambia a EXPIRED las oportunidades en curso cuya fecha de expiracion ya paso
func ProcessExpiredOpportunities() {
	expired, err := services.ExpireOpportunitiesService(time.Now().UTC())
	if !err.IsEmtpy() {
		log.Printf("Error expirando las oportunidades, se reintentara en el siguiente ciclo: %v", err)
		return
	}

	log.Printf("Oportunidades expiradas: %d", expired)
}
