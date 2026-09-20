from qdrant_client import models

from app.rag.errors import RagError


async def upsert_verify_delete(client, collection: str, points: list[models.PointStruct], existing_ids: set, batch_size: int = 64):
    """调用方负责互斥和范围校验；任何批次失败都不得提前删除旧点。"""
    expected = {point.id: point.payload for point in points}
    if len(expected) != len(points):
        raise RagError("Duplicate expected point identities")
    for start in range(0, len(points), batch_size):
        await client.upsert(collection, points=points[start:start + batch_size], wait=True)
    ids = list(expected)
    for start in range(0, len(ids), batch_size):
        batch = ids[start:start + batch_size]
        records = await client.retrieve(collection, ids=batch, with_payload=True, with_vectors=False)
        if len(records) != len(batch) or {record.id for record in records} != set(batch):
            raise RagError("Cloud index verification failed; explicit retry is required")
        if any(record.payload != expected[record.id] for record in records):
            raise RagError("Cloud index payload verification failed; explicit retry is required")
    stale = sorted(existing_ids - set(expected), key=str)
    for start in range(0, len(stale), batch_size):
        await client.delete(collection, points_selector=models.PointIdsList(points=stale[start:start + batch_size]), wait=True)
