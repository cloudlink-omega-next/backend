package database

import (
	"fmt"
	"math"

	"github.com/cloudlink-omega/accounts/pkg/constants"
	"github.com/cloudlink-omega/storage/pkg/bitfield"
	"github.com/cloudlink-omega/storage/pkg/types"
	"gorm.io/gorm"
)

func (d *Database) GameExists(id string) bool {
	res := d.DB.Where("id = ?", id).First(&types.DeveloperGame{})

	return res.RowsAffected > 0 && res.Error == nil
}

func (d *Database) GetGame(id string) (game *types.DeveloperGame) {
	if game, ok := d.Cache.Get("game", id); ok {
		return game.(*types.DeveloperGame)
	}

	res := d.DB.Preload("Developer").Preload("Developer.DeveloperMembers").Preload("Features").Where("id = ?", id).First(&game)
	if res.Error != nil {
		if res.Error == gorm.ErrRecordNotFound {
			return nil
		} else {
			panic(res.Error)
		}
	}

	d.Cache.Set("game", game, id)
	return game
}

type cached_all struct {
	Games    []*types.DeveloperGame
	Total    int64
	MaxPages int64
}

func (d *Database) GetAllGames(page int, limit int) (games []*types.DeveloperGame, total int64, max_pages int64) {
	cache_key := fmt.Sprintf("page_%d:limit_%d", page, limit)

	if games, ok := d.Cache.Get("allgames", cache_key); ok {
		return games.(cached_all).Games, games.(cached_all).Total, games.(cached_all).MaxPages
	}

	var bitfield bitfield.Bitfield8
	bitfield.ManySet(constants.GAME_IS_ACTIVE, constants.GAME_IS_VERIFIED)

	d.DB.Preload("Developer").Preload("Features").Preload("Thumbnail").Limit(limit).Offset(page*limit).Where("state >= ?", bitfield).Order("created_at DESC").Find(&games)
	d.DB.Find(&types.DeveloperGame{}).Count(&total)
	max_pages = int64(math.Ceil(float64(total) / float64(limit)))

	d.Cache.Set("allgames", cached_all{games, total, max_pages}, cache_key)
	return games, total, max_pages
}

// GetGamesForDeveloper returns every game owned by a developer account, including
// submissions that are still awaiting review.
func (d *Database) GetGamesForDeveloper(developer_id string) (games []*types.DeveloperGame) {
	d.DB.Preload("Features").Where("developer_id = ?", developer_id).Order("created_at DESC").Find(&games)
	return games
}

// GetPendingGames returns every submitted game that is neither published nor rejected.
func (d *Database) GetPendingGames() (games []*types.DeveloperGame) {
	var published bitfield.Bitfield8
	published.ManySet(constants.GAME_IS_ACTIVE, constants.GAME_IS_VERIFIED)

	var rejected bitfield.Bitfield8
	rejected.Set(constants.GAME_WAS_REJECTED)

	d.DB.Preload("Developer").
		Where("state & ? = 0", uint8(published)).
		Where("state & ? = 0", uint8(rejected)).
		Order("created_at ASC").
		Find(&games)
	return games
}
