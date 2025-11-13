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
local function abs(n) return (n < 0) and -n or n end

function getNextDirection(aroundPlayerHead)
    -- The map is always centered on the snake's head.
    -- Assuming the map is N x N, the center index is (N+1)/2 (Lua is 1-indexed).
    local size = #aroundPlayerHead
    local center = math.floor((size + 1) / 2)
    local head_y = center
    local head_x = center

    local tail_dy, tail_dx = 0, 0
    
    -- 1. Find the tail's location and calculate direction from head to tail
    for y = 1, size do
        for x = 1, #aroundPlayerHead[y] do
            local tile = aroundPlayerHead[y][x]
            if tile.IsTail then
                tail_dy = y - head_y -- Relative movement in Y (row)
                tail_dx = x - head_x -- Relative movement in X (column)
                break
            end
        end
        if tail_dy ~= 0 or tail_dx ~= 0 then break end
    end

    -- 2. Check for adjacency (Manhattan distance of 1: Up, Down, Left, or Right)
    if (abs(tail_dy) + abs(tail_dx) == 1) then
        -- This implements the "close the loop" move.
        -- We move directly into the adjacent tail tile.
        return {Dy=tail_dy, Dx=tail_dx}
    end

    -- 3. Default Strategy: Try to continue straight.
    local headTile = aroundPlayerHead[head_y][head_x]
    local current_dy = headTile.Direction.Dy
    local current_dx = headTile.Direction.Dx
    
    if current_dy ~= 0 or current_dx ~= 0 then
        local next_y = head_y + current_dy
        local next_x = head_x + current_dx

        -- Check if the move is within the local map bounds
        if next_y >= 1 and next_y <= size and next_x >= 1 and next_x <= #aroundPlayerHead[next_y] then
            local nextTile = aroundPlayerHead[next_y][next_x]
            -- Move straight if the next tile is empty (OwnerColor == nil) or is the tail (already handled, but safe to check)
            if nextTile.OwnerColor == nil or nextTile.IsTail then
                return {Dy=current_dy, Dx=current_dx}
            end
        end
    end

    -- 4. Fallback: If no other move is safe, try moving North (Dy=-1)
    return {Dy=-1, Dx=0}
end
		`,
	}
}

func getBotsNextDirection(botPlayer *Player, gm *GameManager) (Direction, error) {
	botStrategy := getBotStrategy(botPlayer.StrategyName)
	if botStrategy == nil {
		return Direction{}, errors.New("bot strategy not found")
	}

	luaState := lua.NewState()
	defer luaState.Close()
	if err := luaState.DoString(botStrategy.StrategyDefinition); err != nil {
		return Direction{}, errors.New("could not parse lua strategy definition")
	}

	luaFunctionUncasted := luaState.GetGlobal("getNextDirection")
	luaFunction, ok := luaFunctionUncasted.(*lua.LFunction)
	if !ok {
		return Direction{}, fmt.Errorf("lua global 'getNextDirection' was type %s, expected function", luaFunctionUncasted.Type().String())
	}

	aroundPlayersHeadMap := getMapAroundHead(botPlayer, gm)
	luaState.Push(luaFunction)

	luaState.Push(convertGoMapToLuaTable(luaState, aroundPlayersHeadMap))

	if err := luaState.PCall(1, 1, nil); err != nil {
		return Direction{}, fmt.Errorf("could not execute lua strategy definition: %w", err)
	}

	if luaState.GetTop() < 1 {
		return Direction{}, errors.New("lua function executed but did not return any value")
	}
	luaReturn := luaState.Get(-1)

	luaTable, ok := luaReturn.(*lua.LTable)
	if !ok {
		return Direction{}, fmt.Errorf("lua return value was type %s, expected table", luaReturn.Type().String())
	}

	ret := convertLuaDirectionTableToGoStruct(luaTable)

	luaState.Pop(1)
	return ret, nil
}

func convertLuaDirectionTableToGoStruct(luaTbl *lua.LTable) Direction {
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

func convertTileToLuaTable(L *lua.LState, tile Tile) *lua.LTable {
	tbl := L.NewTable()
	if tile.OwnerColor != nil {
		tbl.RawSetString("OwnerColor", lua.LNumber(*tile.OwnerColor))
	} else {
		tbl.RawSetString("OwnerColor", lua.LNil)
	}

	tbl.RawSetString("IsHead", lua.LBool(tile.IsHead))
	tbl.RawSetString("IsTail", lua.LBool(tile.IsTail))
	tbl.RawSetString("X", lua.LNumber(tile.X))
	tbl.RawSetString("Y", lua.LNumber(tile.Y))

	directionTable := L.NewTable()
	directionTable.RawSetString("Dy", lua.LNumber(tile.Direction.Dy))
	directionTable.RawSetString("Dx", lua.LNumber(tile.Direction.Dx))
	tbl.RawSetString("Direction", directionTable)

	return tbl
}

func convertGoMapToLuaTable(luaState *lua.LState, aroundPlayersHead [][]Tile) *lua.LTable {
	luaMapTable := luaState.NewTable()
	for _, row := range aroundPlayersHead {
		luaRowTable := luaState.NewTable()
		for _, cellValue := range row {
			luaTileTable := convertTileToLuaTable(luaState, cellValue)
			luaRowTable.Append(luaTileTable)
		}

		luaMapTable.Append(luaRowTable)
	}

	return luaMapTable
}
