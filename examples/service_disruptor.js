import { ServiceDisruptor } from 'k6/x/disruptor';

const fault = {
        average_delay: 100,
        error_rate: 0.1,
        error_code: 500
}

export default function() {
    const disruptor = new ServiceDisruptor("nginx", "default")
    const targets = disruptor.targets()
    if (targets.length != 1) {
      throw new Error("expected list to have one target")
    }
    disruptor.injectHTTPFaults(fault, 30)
}
