<?php

use Illuminate\Support\Facades\Artisan;

Artisan::command('metergate:mode', function (): void {
    $this->info(config('metergate.mode'));
});

