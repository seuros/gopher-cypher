MATCH (n:User) WHERE n.age > 30 RETURN n.name
MATCH (u:User) RETURN u.email LIMIT 10
MATCH (p:Product) WHERE p.price < 100 RETURN p.name, p.price SKIP 5 LIMIT 20