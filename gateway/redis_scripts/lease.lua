local fleet_key = KEYS[1]
local amount = tonumber(ARGV[1])

local current = tonumber(redis.call('GET', fleet_key) or "0")
if current >= amount then
    redis.call('DECRBY', fleet_key, amount)
    return amount
elseif current > 0 then
    redis.call('SET', fleet_key, 0)
    return current
else
    return 0
end
