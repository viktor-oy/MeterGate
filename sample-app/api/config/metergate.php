<?php

return [
    'mode' => env('METERGATE_MODE', 'proxy'),
    'provider_url' => env('METERGATE_PROVIDER_URL', 'http://metergate:3000/v1/check'),
    'api_key_header' => env('METERGATE_API_KEY_HEADER', 'x-api-key'),
    'otel_file_path' => env('OTEL_FILE_PATH', storage_path('logs/sample-api-otel.ndjson')),
];

