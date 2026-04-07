CREATE INDEX name_idx
ON profile ((LOWER(name)) text_pattern_ops);

CREATE INDEX surname_idx
ON profile ((LOWER(surname)) text_pattern_ops);