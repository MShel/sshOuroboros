package game

import (
	"errors"

	lua "github.com/yuin/gopher-lua"
)

type BotStrategy struct {
	StrategyName       string
	StrategyDefinition string
}

func getBotStrategy(name string) *BotStrategy {
	return &BotStrategy{
		StrategyName: "MoveNorth",
		StrategyDefinition: `
			function getNextDirection(playerHead, mapAroundHead)
				return {Dy=0, Dx=-1}
			end
		`,
	}
}

func getBotsNextDirection(botPlayer *Player) (error, Direction) {
	botStrategy := getBotStrategy(botPlayer.StrategyName)
	if botStrategy == nil {
		return errors.New("bot strategy not found"), Direction{}
	}

	luaState := lua.NewState()
	defer luaState.Close()
	if err := luaState.DoString(botStrategy.StrategyDefinition); err != nil {
		return errors.New("could not parse lua strategy definition"), Direction{}
	}

	luaState.GetGlobal("getNextDirection")
	// TODO add player info to table
	luaState.Push(&lua.LTable{})
	if err := luaState.PCall(1, 1, nil); err != nil {
		return errors.New("could not execute lua strategy definition"), Direction{}
	}

	luaReturn := luaState.Get(-1)
	luaTable, ok := luaState.Get(-1).(*lua.LTable)
	if !ok {
		return errors.New("Error: Lua return value was type" + luaReturn.Type().String() + ", expected table.\n"), Direction{}
	}

	ret := convertLuaDirectionTableToGoStruct(luaTable)

	luaState.Pop(1)
	return nil, ret
}

func convertLuaDirectionTableToGoStruct(luaTbl *lua.LTable) Direction {
	// Iterate over all key-value pairs in the Lua table
	result := Direction{}
	luaTbl.ForEach(func(key, value lua.LValue) {
		if key.Type() != lua.LTString {
			return
		}

		keyStr := lua.LVAsString(key)

		switch keyStr {
		case "Dy":
			result.Dy = int(lua.LVAsNumber(value))
		case "Dx":
			result.Dx = int(lua.LVAsNumber(value))
		default:
			// Optionally ignore or log unknown keys
		}
	})
	return result
}

func getMapAroundHead(player *Player, gm *GameManager) [][]Tile {
	topRow := player.Location.X - TileRowCountForBotStrategy/2
	topCol := player.Location.Y - TileColCountForBotStrategy/2
	bottomRow := player.Location.X + TileRowCountForBotStrategy/2
	bottomCol := player.Location.Y + TileColCountForBotStrategy/2

	return gm.GetMapCopy(topRow, bottomRow, topCol, bottomCol)
}
