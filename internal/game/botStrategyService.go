package game

import (
	"errors"
	"fmt"

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
			function getNextDirection()
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

	luaFunctionUncasted := luaState.GetGlobal("getNextDirection")
	luaFunction, ok := luaFunctionUncasted.(*lua.LFunction)
	if !ok {
		return fmt.Errorf("lua return value was type %s, function", luaFunction.Type().String()), Direction{}
	}

	luaState.Push(luaFunction)

	if err := luaState.PCall(0, 1, nil); err != nil {
		return fmt.Errorf("could not execute lua strategy definition: %w", err), Direction{}
	}

	if luaState.GetTop() < 1 {
		return errors.New("Lua function executed but did not return any value."), Direction{}
	}
	luaReturn := luaState.Get(-1)

	luaTable, ok := luaReturn.(*lua.LTable)
	if !ok {
		return fmt.Errorf("Lua return value was type %s, expected table", luaReturn.Type().String()), Direction{}
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
