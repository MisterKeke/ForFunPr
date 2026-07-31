using System.Text.Json;
using Something.Domain.Exceptions;

namespace Something.Infrastructure.Providers;

internal static class BoundedHttpContent
{
    private static readonly JsonSerializerOptions JsonOptions = new(JsonSerializerDefaults.Web);

    public static async Task<T> ReadJsonAsync<T>(
        HttpResponseMessage response,
        int maximumBytes,
        string providerName,
        CancellationToken cancellationToken)
    {
        var mediaType = response.Content.Headers.ContentType?.MediaType;
        if (mediaType is not null &&
            !mediaType.Equals("application/json", StringComparison.OrdinalIgnoreCase) &&
            !mediaType.EndsWith("+json", StringComparison.OrdinalIgnoreCase))
        {
            throw new ProviderUnavailableException($"{providerName} returned an unexpected response.");
        }

        var bytes = await ReadBytesAsync(response, maximumBytes, providerName, cancellationToken);
        try
        {
            return JsonSerializer.Deserialize<T>(bytes, JsonOptions)
                ?? throw new ProviderUnavailableException($"{providerName} returned an empty response.");
        }
        catch (JsonException exception)
        {
            throw new ProviderUnavailableException(
                $"{providerName} returned an invalid response.",
                exception);
        }
    }

    public static async Task<byte[]> ReadBytesAsync(
        HttpResponseMessage response,
        int maximumBytes,
        string providerName,
        CancellationToken cancellationToken)
    {
        var contentLength = response.Content.Headers.ContentLength;
        if (contentLength > maximumBytes)
        {
            throw new ProviderUnavailableException($"{providerName} returned too much data.");
        }

        await using var input = await response.Content.ReadAsStreamAsync(cancellationToken);
        using var output = new MemoryStream(Math.Min(maximumBytes, 32 * 1024));
        var buffer = new byte[8192];
        while (true)
        {
            var read = await input.ReadAsync(buffer, cancellationToken);
            if (read == 0)
            {
                break;
            }

            if (output.Length + read > maximumBytes)
            {
                throw new ProviderUnavailableException($"{providerName} returned too much data.");
            }

            output.Write(buffer, 0, read);
        }

        return output.ToArray();
    }
}
