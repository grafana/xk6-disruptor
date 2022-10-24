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
        average_delay: 100,
        error_rate: 0.1,
        error_code: 500
}

export default function () {
        const disruptor = new PodDisruptor(selector)
        const targets = disruptor.targets()
        if (targets.length != 1) {
        	throw new Error("expected list to have one target")
        }

       disruptor.injectHTTPFaults(fault, 30)
}