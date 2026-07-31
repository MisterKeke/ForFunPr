namespace Something.Infrastructure.Caching;

internal sealed class BoundedTtlCache<T>(int capacity, TimeSpan lifetime)
{
    private readonly object _gate = new();
    private readonly Dictionary<string, Entry> _entries = new(StringComparer.Ordinal);
    private readonly int _capacity = Math.Max(1, capacity);
    private readonly TimeSpan _lifetime = lifetime > TimeSpan.Zero ? lifetime : TimeSpan.FromMinutes(1);
    private long _accessSequence;

    public bool TryGet(string key, out T value)
    {
        lock (_gate)
        {
            RemoveExpired(DateTimeOffset.UtcNow);
            if (_entries.TryGetValue(key, out var entry))
            {
                entry = entry with { LastAccess = ++_accessSequence };
                _entries[key] = entry;
                value = entry.Value;
                return true;
            }
        }

        value = default!;
        return false;
    }

    public void Set(string key, T value)
    {
        lock (_gate)
        {
            var now = DateTimeOffset.UtcNow;
            RemoveExpired(now);
            if (!_entries.ContainsKey(key) && _entries.Count >= _capacity)
            {
                var leastRecentlyUsed = _entries.MinBy(static pair => pair.Value.LastAccess);
                _entries.Remove(leastRecentlyUsed.Key);
            }

            _entries[key] = new Entry(value, now + _lifetime, ++_accessSequence);
        }
    }

    public void Invalidate(string key)
    {
        lock (_gate)
        {
            _entries.Remove(key);
        }
    }

    private void RemoveExpired(DateTimeOffset now)
    {
        foreach (var key in _entries
                     .Where(pair => pair.Value.ExpiresAt <= now)
                     .Select(static pair => pair.Key)
                     .ToArray())
        {
            _entries.Remove(key);
        }
    }

    private sealed record Entry(T Value, DateTimeOffset ExpiresAt, long LastAccess);
}
