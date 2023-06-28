package cache

// Removes all elements from different sorted sets
//
// KEYS[1-n] -> sorted sets to remove elements
//
// ARGV[1/n] -> elements to remove from all sorted sets
const removeElementScript = `
local total = 0

for _, pool in ipairs(KEYS) do
	total = total + redis.call('ZREM', pool, unpack(ARGV))
end

return total
`

// Moves an element from a sorted set to another setting a specific score
//
// KEYS[1] -> destination sorted set
//
// KEYS[2] -> source sorted set
//
// ARGV[1] -> the score
//
// ARGV[2] -> the element to be moved
const moveElementScript = `
local removed = redis.call('ZREM', KEYS[2], ARGV[2])

if removed > 0 then
	return redis.call('ZADD', KEYS[1], unpack(ARGV))
end

return 0
`

// Adds all elements into a sorted set if they are not present in any other sorted set
//
// KEYS[1] -> destination sorted set
//
// KEYS[1-n] -> sorted sets to check element presence
//
// ARGV[1-n * 2] -> elements to be added (score, id)
const addElementsScript = `
local toInsert = {}
local inserted = {}

for i=1, #ARGV, 2 do
	local score = ARGV[i]
	local element = ARGV[i+1]
	local exists = false

	for _, pool in ipairs(KEYS) do
		local score = redis.call('ZSCORE', pool, element)

		if score then
			exists = true

			break
		end
	end

	if not exists then
		table.insert(toInsert, score)
		table.insert(toInsert, element)
		table.insert(inserted, element)
	end
end

if table.getn(toInsert) == 0 then
	return inserted
end

redis.call('ZADD', KEYS[1], unpack(toInsert))

return inserted
`

// Checks if any sorted set contains the provided element
//
// KEYS[1-n] -> sorted sets to check for elements
//
// ARGV[1] -> element to check
const containsElementScript = `
for _, pool in ipairs(KEYS) do
	local score = redis.call('ZSCORE', pool, ARGV[1])

	if score then
		return 1
	end
end

return 0
`

// Moves elements from a sorted set filtered by a score into the destination sorted set
//
// KEYS[1] -> destination sorted set
//
// KEYS[2] -> source sorted set
//
// ARGV[1] -> the score to filter elements to move
//
// ARGV[2] -> the score to set in the new sorted set
//
// ARGV[3] -> number of elements to move
const moveFilteredElementsScript = `
local elements = redis.call('ZREVRANGEBYSCORE', KEYS[2], ARGV[1], '0', 'LIMIT', '0', tostring(ARGV[3]))
if next(elements) == nil then
    return ''
end
for i, key in ipairs(elements) do
	redis.call('ZADD', KEYS[1], ARGV[2], key)
end
redis.call('ZREM', KEYS[2], unpack(elements))

return elements
`

// Moves n elements from a sorted set into another sorted set and return moved elements.
//
// KEYS[1] -> sorted set to move from
//
// KEYS[2] -> sorted set to move to
//
// ARGV[1] -> number of elements to move
//
// ARGV[2] -> value to use as new score in the destination sorted set
//
// ARGV[3] -> value to filter elements by score
const pullElementsScript = `
local elements
if ARGV[3] == '0' then
	elements = redis.call('ZRANGE', KEYS[1], '0', tostring(tonumber(ARGV[1]) - 1))
else
	elements = redis.call('ZRANGEBYSCORE', KEYS[1], '0', ARGV[3], 'LIMIT', '0', tostring(ARGV[1]))
end
if next(elements) == nil then
    return ''
end
for i, key in ipairs(elements) do
	redis.call('ZADD', KEYS[2], ARGV[2], key)
end
redis.call('ZREM', KEYS[1], unpack(elements))
return elements
`
