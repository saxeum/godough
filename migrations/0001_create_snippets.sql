CREATE TABLE snippets (
    id      integer GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    title   varchar(100) NOT NULL,
    content text NOT NULL,
    created timestamptz NOT NULL,
    expires timestamptz NOT NULL
);

CREATE INDEX idx_snippets_created ON snippets (created);

INSERT INTO snippets (title, content, created, expires) VALUES (
    'An old silent pond',
    E'An old silent pond...\nA frog jumps into the pond,\nsplash! Silence again.\n\n– Matsuo Bashō',
    now(),
    now() + interval '365 days'
);

INSERT INTO snippets (title, content, created, expires) VALUES (
    'Over the wintry forest',
    E'Over the wintry\nforest, winds howl in rage\nwith no leaves to blow.\n\n– Natsume Soseki',
    now(),
    now() + interval '365 days'
);

INSERT INTO snippets (title, content, created, expires) VALUES (
    'First autumn morning',
    E'First autumn morning\nthe mirror I stare into\nshows my father''s face.\n\n– Murakami Kijo',
    now(),
    now() + interval '7 days'
);
