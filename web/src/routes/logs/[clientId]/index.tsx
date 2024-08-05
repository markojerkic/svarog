import { type RouteDefinition, useParams } from "@solidjs/router";
import { createLogViewer } from "~/components/log-viewer";
import { createLogSubscription } from "~/lib/store/connection";
import { createLogQuery } from "~/lib/store/log-store";

export const route = {
    load: async ({ params }) => {
        const clientId = params.clientId;

        const logData = createLogQuery(clientId);
        return await logData.fetchPreviousPage();
    },
} satisfies RouteDefinition;

export default () => {
    const clientId = useParams<{ clientId: string }>().clientId;
    const logs = createLogQuery(clientId);

    const [LogViewer, scrollToBottom] = createLogViewer();

    createLogSubscription(clientId, logs.logStore, scrollToBottom);

    return (
        <div class="flex justify-start gap-2">
            <div class="h-full flex-shrink">Tu će biti odabir instanci</div>
            <div class="flex-grow">
                <LogViewer logsQuery={logs} />
            </div>
        </div>
    );
};
