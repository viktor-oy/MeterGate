import { Module } from '@nestjs/common';
import { CacheSyncService } from './cache-sync.service';

@Module({
  providers: [CacheSyncService]
})
export class SyncModule {}
