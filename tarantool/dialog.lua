local clock = require('clock')
local uuid  = require('uuid')

-- same canonical key as Go: lexicographically smaller uuid first
local function conversation_id(a, b)
    if a < b then
        return a .. ':' .. b
    else
        return b .. ':' .. a
    end
end

-- dialog_send(args) -> tuple of message
-- args: { from_user_id, to_user_id, text }
function dialog_send(args)
    local from_user_id = args[1]
    local to_user_id   = args[2]
    local text         = args[3]

    if from_user_id == nil or to_user_id == nil or text == nil or text == '' then
        error('invalid arguments')
    end
    if from_user_id == to_user_id then
        error('cannot send message to yourself')
    end

    local id   = uuid.str()
    local c_id = conversation_id(from_user_id, to_user_id)
    local ts   = clock.realtime() * 1000   -- milliseconds as number

    box.space.messages:insert{ id, c_id, from_user_id, to_user_id, text, ts }

    return {
        id              = id,
        conversation_id = c_id,
        from_user_id    = from_user_id,
        to_user_id      = to_user_id,
        text            = text,
        created_at      = ts,
    }
end

-- dialog_list(args) -> array of messages
-- args: { user_a, user_b }
function dialog_list(args)
    local user_a = args[1]
    local user_b = args[2]

    local c_id = conversation_id(user_a, user_b)
    local result = {}

    for _, tuple in box.space.messages.index.conv_created:pairs(c_id, { iterator = 'EQ' }) do
        table.insert(result, {
            id              = tuple[1],
            conversation_id = tuple[2],
            from_user_id    = tuple[3],
            to_user_id      = tuple[4],
            text            = tuple[5],
            created_at      = tuple[6],
        })
    end
    return result
end
