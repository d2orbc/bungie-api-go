package main

import (
	"context"
	"log"
	"os"

	bnet "github.com/d2orbc/bungie-api-go"
)

func main() {
	apiKey := os.Getenv("BUNGIE_API_KEY")
	api := bnet.NewAPI(apiKey)

	ctx := context.Background()
	resp, err := api.Destiny2GetPostGameCarnageReport(ctx,
		bnet.Destiny2GetPostGameCarnageReportRequest{
			ActivityID: 10_000_000_000,
		})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Raw: %s", resp.Raw())
	log.Println()
	for _, entry := range resp.Response.Entries {
		log.Print(entry.Player.DestinyUserInfo.BungieGlobalDisplayName)
	}
	log.Print(bnet.Version())

	log.Print("Searching")

	searchResp, err := api.UserSearchByGlobalNamePost(ctx, bnet.UserSearchByGlobalNamePostRequest{
		Page: 0,
		Body: bnet.UserSearchPrefixRequest{
			DisplayNamePrefix: "cbro",
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	for _, r := range searchResp.Response.SearchResults {
		log.Printf("%s#%04d", r.BungieGlobalDisplayName, r.BungieGlobalDisplayNameCode.Must())

		if r.BungieGlobalDisplayNameCode.Must() == 7109 {
			log.Print("found cbro#7109")
			profile, err := api.Destiny2GetProfile(ctx, bnet.Destiny2GetProfileRequest{
				Components: []bnet.ComponentType{
					bnet.ComponentType_Characters,
				},
				MembershipType:      r.DestinyMemberships[0].MembershipType,
				DestinyMembershipID: r.DestinyMemberships[0].MembershipID,
			})
			if err != nil {
				log.Fatal(err)
			}

			log.Print(profile.ErrorStatus, profile.Message)
			for _, char := range profile.Response.Characters.Data {
				classDef, _ := char.ClassHash.Fetch(api)
				log.Print(classDef.DisplayProperties.Name)

				emblemDef, _ := char.EmblemHash.Fetch(api)
				log.Print(emblemDef.DisplayProperties.Name)
			}
		}
	}
}
