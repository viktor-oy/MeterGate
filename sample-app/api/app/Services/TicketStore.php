<?php

namespace App\Services;

final class TicketStore
{
    /** @var array<int, array{id: string, subject: string, status: string, comments: array<int, array{id: string, body: string, createdAt: string}>}> */
    private static array $tickets = [
        [
            'id' => 'TCK-1001',
            'subject' => 'Webhook retries delayed',
            'status' => 'open',
            'comments' => [
                ['id' => 'C-1', 'body' => 'Customer reports retries after three minutes.', 'createdAt' => '2025-11-18T10:00:00Z'],
            ],
        ],
        [
            'id' => 'TCK-1002',
            'subject' => 'Invoice export missing metadata',
            'status' => 'triage',
            'comments' => [
                ['id' => 'C-2', 'body' => 'Reproduced with CSV export in provider mode.', 'createdAt' => '2025-11-18T10:20:00Z'],
            ],
        ],
    ];

    /** @return array<int, array<string, mixed>> */
    public function all(): array
    {
        return array_values(self::$tickets);
    }

    /** @return array<string, mixed>|null */
    public function find(string $id): ?array
    {
        foreach (self::$tickets as $ticket) {
            if ($ticket['id'] === $id) {
                return $ticket;
            }
        }

        return null;
    }

    /** @return array<string, mixed> */
    public function addComment(string $ticketId, string $body): array
    {
        foreach (self::$tickets as &$ticket) {
            if ($ticket['id'] === $ticketId) {
                $ticket['comments'][] = [
                    'id' => 'C-'.(count($ticket['comments']) + 1).'-'.substr(md5($body), 0, 4),
                    'body' => $body,
                    'createdAt' => gmdate('c'),
                ];

                return $ticket;
            }
        }

        throw new \RuntimeException("Ticket {$ticketId} not found");
    }
}

