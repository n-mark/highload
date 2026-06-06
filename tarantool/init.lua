box.cfg {
    listen = 3301,
    memtx_memory = 512 * 1024 * 1024,
    wal_mode = 'write',
    log_level = 5,
}

-- application user
box.once('schema:v1', function()
    box.schema.user.create('app', { password = 'app_pass' })
    box.schema.user.grant('app', 'super')
end)

-- messages space
box.once('messages:v1', function()
    local msgs = box.schema.space.create('messages', { if_not_exists = true })
    msgs:format({
        { name = 'id',              type = 'string' },
        { name = 'conversation_id', type = 'string' },
        { name = 'from_user_id',    type = 'string' },
        { name = 'to_user_id',      type = 'string' },
        { name = 'text',            type = 'string' },
        { name = 'created_at',      type = 'number' },
    })
    msgs:create_index('primary', {
        parts = { 'id' },
        if_not_exists = true,
    })
    msgs:create_index('conv_created', {
        parts = { 'conversation_id', 'created_at' },
        unique = false,
        if_not_exists = true,
    })
end)

-- load dialog procedures
require('dialog')

box.schema.user.grant('app', 'execute', 'universe')
