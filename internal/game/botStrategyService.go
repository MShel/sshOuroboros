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
local function is_safe(tile, head_color)
    -- A move is safe if the tile is NIL (empty) or is our OWN Tail
    return tile.OwnerColor == nil or (tile.IsTail and tile.OwnerColor == head_color)
end

function getNextDirection(aroundPlayerHead)
    local size = #aroundPlayerHead
    -- Assuming a square map, the center index is size/2 + 0.5 (for odd sizes)
    local center_idx = math.floor((size + 1) / 2)
    local headTile = aroundPlayerHead[center_idx][center_idx]
    
    local current_dy = headTile.Direction.Dy
    local current_dx = headTile.Direction.Dx
    local head_color = headTile.OwnerColor

    -- Define movement priority for Square-Making: Straight -> Right Turn -> Left Turn
    local preferredDirections = {
        -- 1. Straight (Continue current side)
        {Dy=current_dy, Dx=current_dx},
        -- 2. Right Turn (Change to next side of the square: (Dx, Dy) -> (-Dy, Dx))
        {Dy=-current_dx, Dx=current_dy}, 
        -- 3. Left Turn (Fallback turn)
        {Dy=current_dx, Dx=-current_dy}, 
    }
    
    -- 1. HIGHEST PRIORITY: CLOSE THE LOOP (Adjacent Tail)
    -- Check all 4 cardinal directions for an adjacent own tail tile
    local cardinalDirections = {
        {Dy=-1, Dx=0}, {Dy=1, Dx=0}, {Dy=0, Dx=1}, {Dy=0, Dx=-1}
    }

    for _, dir in ipairs(cardinalDirections) do
        local next_y = center_idx + dir.Dy
        local next_x = center_idx + dir.Dx
        
        -- Boundary check (must be within the small local map)
        if next_y >= 1 and next_y <= size and next_x >= 1 and next_x <= #aroundPlayerHead[next_y] then
            local nextTile = aroundPlayerHead[next_y][next_x]
            
            -- Check if this move is a loop-closer AND not a 180-degree turn
            if nextTile.IsTail and nextTile.OwnerColor == head_color then
                -- This is the highest priority move
                return dir
            end
        end
    end

    -- 2. FOLLOW SQUARE PATH (Straight -> Right Turn)
    for _, dir in ipairs(preferredDirections) do
        local next_y = center_idx + dir.Dy
        local next_x = center_idx + dir.Dx
        
        -- CRITICAL: Check for 180-degree turn
        if dir.Dy == -current_dy and dir.Dx == -current_dx then
            goto continue
        end

        -- Check map bounds
        if next_y >= 1 and next_y <= size and next_x >= 1 and next_x <= #aroundPlayerHead[next_y] then
            local nextTile = aroundPlayerHead[next_y][next_x]
            
            -- If the move is safe (Empty or Own Tail)
            if is_safe(nextTile, head_color) then
                return dir
            end
        end
        
        ::continue::
    end

    -- 3. FINAL FALLBACK: Return current direction 
    return {Dy=current_dy, Dx=current_dx} 
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

	fmt.Printf("\n %v\n", aroundPlayersHeadMap)
	//panic("derp")

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
	fmt.Printf("%v", ret)
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
	topRow := player.Location.Y - TileRowCountForBotStrategy/2
	bottomRow := player.Location.Y + TileRowCountForBotStrategy/2

	topCol := player.Location.X - TileColCountForBotStrategy/2
	bottomCol := player.Location.X + TileColCountForBotStrategy/2

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
