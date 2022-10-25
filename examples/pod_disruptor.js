import { PodDisruptor } from 'k6/x/disruptor';

const selector = {
        namespace: "default",
        select: {
                labels: {
                        run: "nginx"
                }
        }
}

const fault = {
        averageDelay: 100,
        errorRate: 0.1,
        errorCode: 500
}

export default function () {
        const disruptor = new PodDisruptor(selector)
        const targets = disruptor.targets()
        if (targets.length != 1) {
        	throw new Error("expected list to have one target")
        }

       disruptor.injectHTTPFaults(fault, 30)
}