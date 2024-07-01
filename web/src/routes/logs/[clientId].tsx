import { RouteDefinition, cache, createAsync, useParams } from "@solidjs/router"
import { createVirtualizer } from "@tanstack/solid-virtual"
import { For, Match, Switch, createEffect, onMount } from "solid-js"

type LogLine = {
    ID: string
    LogLine: string
    LogLevel: number
    Timestamp: string
    Client: {
        ClientId: string
        IpAddress: string
    }
    SequenceNumber: number
}

const getLogsPage = cache(async (clientId: string, cursorTime: number, cursorId: string) => {
    const response = await fetch(`http://localhost:1323/api/v1/logs/${clientId}`)
    return response.json() as Promise<LogLine[]>
}, "logs")

export const route = {
    load: ({ params }) => {
        const clientId = params.clientId
        console.log("Loading logs for client", clientId)
        return getLogsPage(clientId, 0, "")
    }
} satisfies RouteDefinition

export default () => {
    let logsRef: HTMLDivElement | undefined = undefined

    const clientId = useParams<{ clientId: string }>().clientId
    const logs = createAsync(() => getLogsPage(clientId, 0, ""))
    const logsOrEmpty = () => logs() ?? []

    const virtualizer = createVirtualizer({
        get count() {
            return logs()?.length ?? 0
        },
        estimateSize: () => 25,
        getScrollElement: () => logsRef
    })


    onMount(() => {
        logsRef?.scrollTo(0, logsRef.scrollHeight)
    })

    return (
        <>
            <div
                ref={logsRef}
                style={{
                    height: "90vh",
                    width: `100%`,
                    overflow: 'auto',
                }}
            >
                <div
                    style={{
                        height: `${virtualizer.getTotalSize()}px`,
                        width: '100%',
                        position: 'relative',
                    }}
                >
                    <For each={virtualizer.getVirtualItems()}>
                        {(virtualItem) => {
                            console.log("Tu sam ");
                            return (
                                <div
                                    style={{
                                        position: 'absolute',
                                        top: 0,
                                        left: 0,
                                        width: '100%',
                                        height: `${virtualItem.size}px`,
                                        transform: `translateY(${virtualItem.start}px)`,
                                    }}
                                >
                                    <pre class="text-white">
                                        {logsOrEmpty()[virtualItem.index].LogLine}
                                    </pre>
                                </div>
                            );
                        }}
                    </For>
                </div>
            </div>

        </>
    )
}
