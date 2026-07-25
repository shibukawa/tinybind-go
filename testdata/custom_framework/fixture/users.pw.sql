package fixture

type UserRow { id: int, name: string }

export statement FindUser(id: int): sql.one<UserRow> {SELECT id, name FROM users WHERE id = {id}}
