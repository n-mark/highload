package store

import (
    "context"
    "errors"

    "github.com/google/uuid"
)

var (
    ErrFriendNotFound = errors.New("friend not found")
    ErrAlreadyFriends = errors.New("already friends")
    ErrFriendSelf     = errors.New("cannot add yourself as friend")
)

type FriendStore struct {
    master  DB
    replica DB
}

func NewFriendStore(master, replica DB) *FriendStore {
    return &FriendStore{master: master, replica: replica}
}

func (s *FriendStore) AddFriend(ctx context.Context, userID, friendID uuid.UUID) error {
    if userID == friendID {
        return ErrFriendSelf
    }

    _, err := s.master.Exec(ctx,
        "INSERT INTO friendship (user_id, friend_id) VALUES ($1, $2) ON CONFLICT DO NOTHING",
        userID, friendID,
    )
    if err != nil {
        return mapPgError(err, ErrAlreadyFriends)
    }
    return nil
}

func (s *FriendStore) DeleteFriend(ctx context.Context, userID, friendID uuid.UUID) error {
    tag, err := s.master.Exec(ctx,
        "DELETE FROM friendship WHERE user_id=$1 AND friend_id=$2",
        userID, friendID,
    )
    if err != nil {
        return err
    }
    if tag.RowsAffected() == 0 {
        return ErrFriendNotFound
    }
    return nil
}

func (s *FriendStore) GetFriendIDs(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
    rows, err := s.replica.Query(ctx,
        "SELECT friend_id FROM friendship WHERE user_id=$1",
        userID,
    )
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var ids []uuid.UUID
    for rows.Next() {
        var id uuid.UUID
        if err := rows.Scan(&id); err != nil {
            return nil, err
        }
        ids = append(ids, id)
    }
    return ids, rows.Err()
}

// GetFollowers returns user IDs that have the given userID as a friend
// (i.e. rows where friend_id = userID).
func (s *FriendStore) GetFollowers(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
    rows, err := s.replica.Query(ctx,
        "SELECT user_id FROM friendship WHERE friend_id=$1",
        userID,
    )
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var ids []uuid.UUID
    for rows.Next() {
        var id uuid.UUID
        if err := rows.Scan(&id); err != nil {
            return nil, err
        }
        ids = append(ids, id)
    }
    return ids, rows.Err()
}
