package cronjobs

import (
	cmm "c_trd/common"
	"c_trd/services"
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ProcessExpiredOpportunities cambia a EXPIRED las oportunidades en curso cuya fecha de expiracion ya paso.
// Retorna cuantas se expiraron
func ProcessExpiredOpportunities() int64 {
	expired, err := services.ExpireOpportunitiesService(time.Now().UTC())
	if !err.IsEmtpy() {
		log.Printf("Error expirando las oportunidades, se reintentara en el siguiente ciclo: %v", err)
		return 0
	}

	log.Printf("Oportunidades expiradas: %d", expired)
	return expired
}

// ProcessFinishedOpportunities cambia a FINISHED las oportunidades activas cuya fecha de expiracion ya paso y guarda
// el precio de mercado de su asset en price_achieved. El precio se consulta una vez por asset; si falla la oportunidad
// se finaliza igual sin precio. Retorna cuantas se finalizaron
func ProcessFinishedOpportunities() int64 {
	now := time.Now().UTC()

	opportunities, err := services.GetFinishedOpportunitiesService(now)
	if !err.IsEmtpy() {
		log.Printf("Error obteniendo las oportunidades por finalizar, se reintentara en el siguiente ciclo: %v", err)
		return 0
	}

	if len(opportunities) == 0 {
		return 0
	}

	ctx, cancel := context.WithTimeout(context.Background(), cmm.ActivationPricesTimeout)
	defer cancel()

	prices := make(map[primitive.ObjectID]*float64)
	finishes := make([]services.OpportunityFinish, 0, len(opportunities))
	for _, opportunity := range opportunities {
		price, fetched := prices[opportunity.IDAsset]
		if !fetched {
			price = assetMarketPrice(ctx, opportunity.IDAsset)
			prices[opportunity.IDAsset] = price
		}

		finishes = append(finishes, services.OpportunityFinish{IDOpportunity: opportunity.ID, PriceAchieved: price})
	}

	finished, err := services.FinishOpportunitiesService(finishes, now)
	if !err.IsEmtpy() {
		log.Printf("Error finalizando las oportunidades, se reintentara en el siguiente ciclo: %v", err)
		return 0
	}

	log.Printf("Oportunidades finalizadas: %d", finished)
	return finished
}
