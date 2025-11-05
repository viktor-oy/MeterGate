<?php

namespace App\Services;

final class OtelFile
{
    /** @param array<string, mixed> $attributes */
    public function span(string $name, string $requestId, array $attributes): void
    {
        $line = json_encode([
            'type' => 'otel.span',
            'service' => 'sample-api',
            'name' => $name,
            'requestId' => $requestId,
            'timestamp' => gmdate('c'),
            'attributes' => $attributes,
        ], JSON_THROW_ON_ERROR);

        file_put_contents(config('metergate.otel_file_path'), $line.PHP_EOL, FILE_APPEND);
    }
}

