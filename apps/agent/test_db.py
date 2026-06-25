import os, asyncio
from psycopg_pool import AsyncConnectionPool

async def test():
    print("Connecting to:", os.environ["DATABASE_URL"])
    pool = AsyncConnectionPool(conninfo=os.environ["DATABASE_URL"], min_size=1, max_size=1)
    await pool.open()
    print("Connected!")
    await pool.close()

asyncio.run(test())
