#!/usr/bin/env python3
"""CPU qLogNEI/qLogNEHVI proposal worker for the saved campaign adapter."""
import math
import resource
import sys
from pathlib import Path
import importlib.util

spec = importlib.util.spec_from_file_location("campaign", Path(__file__).with_name("optuna-campaign.py"))
core = importlib.util.module_from_spec(spec)
spec.loader.exec_module(core)


def kernel(d, batch_shape):
    import torch
    from gpytorch.kernels import MaternKernel, ScaleKernel
    from gpytorch.priors import GammaPrior, LogNormalPrior
    if d < 1:
        raise ValueError("empty input dimensions")
    return ScaleKernel(MaternKernel(nu=2.5, ard_num_dims=d, batch_shape=batch_shape,
        lengthscale_prior=LogNormalPrior(torch.tensor(math.sqrt(2) + math.log(d) / 2, dtype=torch.double),
                                       torch.tensor(math.sqrt(3), dtype=torch.double))),
        outputscale_prior=GammaPrior(2, .15), batch_shape=batch_shape)


def propose(data):
    resource.setrlimit(resource.RLIMIT_CPU, (180, 180))
    import torch
    from botorch.models import SingleTaskGP, ModelListGP
    from botorch.models.transforms import Standardize
    from botorch.fit import fit_gpytorch_mll
    from botorch.optim import optimize_acqf
    from botorch.sampling import SobolQMCNormalSampler
    from botorch.utils.transforms import normalize, unnormalize
    from gpytorch.mlls import ExactMarginalLogLikelihood, SumMarginalLogLikelihood
    from botorch.acquisition.objective import GenericMCObjective
    from botorch.acquisition.multi_objective.objective import IdentityMCMultiOutputObjective
    from botorch.acquisition.logei import qLogNoisyExpectedImprovement
    from botorch.acquisition.multi_objective.logei import qLogNoisyExpectedHypervolumeImprovement
    torch.set_num_threads(1)
    torch.set_num_interop_threads(1)
    torch.manual_seed(data["seed"])
    x, obj, bounds = [torch.tensor(data[key], dtype=torch.double) for key in ("x", "objectives", "bounds")]
    con = None if data["constraints"] is None else torch.tensor(data["constraints"], dtype=torch.double)
    if x.ndim != 2 or not 1 <= x.shape[0] <= 512 or not 1 <= x.shape[1] <= 128 or obj.ndim != 2 or len(x) != len(obj) or bounds.shape != (2, x.shape[1]) or not 1 <= obj.shape[1] <= 8:
        raise ValueError("invalid training dimensions")
    if con is not None and (con.ndim != 2 or len(con) != len(x)):
        raise ValueError("invalid constraint dimensions")
    for tensor in (x, obj, bounds, con):
        if tensor is not None and not torch.isfinite(tensor).all():
            raise ValueError("nonfinite training input")
    if not (bounds[1] > bounds[0]).all():
        raise ValueError("invalid proposal bounds")
    if not ((x >= bounds[0]) & (x <= bounds[1])).all():
        raise ValueError("training input outside bounds")
    xn = normalize(x, bounds)
    pending = None if data["pending"] is None else normalize(torch.tensor(data["pending"], dtype=torch.double), bounds)
    if pending is not None and (pending.ndim != 2 or pending.shape[1] != x.shape[1] or not torch.isfinite(pending).all() or not ((pending >= 0) & (pending <= 1)).all()):
        raise ValueError("invalid pending inputs")
    y = obj if con is None else torch.cat((obj, con), dim=-1)
    nobj, ncon = obj.shape[1], 0 if con is None else con.shape[1]
    constraints = [(lambda z, index=index: z[..., nobj + index]) for index in range(ncon)]
    if nobj == 1:
        _, batch = SingleTaskGP.get_batch_dimensions(train_X=xn, train_Y=y)
        model = SingleTaskGP(xn, y, covar_module=kernel(xn.shape[1], batch), outcome_transform=Standardize(m=y.shape[1]))
        fit_gpytorch_mll(ExactMarginalLogLikelihood(model.likelihood, model))
        acquisition = qLogNoisyExpectedImprovement(model=model, X_baseline=xn,
            sampler=SobolQMCNormalSampler(torch.Size([256])), X_pending=pending,
            objective=GenericMCObjective(lambda z, X: z[..., 0]), constraints=constraints or None)
    else:
        # Constraint GPs are explicit outputs, separate from the objective slice.
        models = [SingleTaskGP(xn, y[:, index:index+1], covar_module=kernel(xn.shape[1], torch.Size()),
                              outcome_transform=Standardize(m=1)) for index in range(y.shape[1])]
        model = ModelListGP(*models)
        fit_gpytorch_mll(SumMarginalLogLikelihood(model.likelihood, model))
        reference = data["reference_point"]
        if not isinstance(reference, list) or len(reference) != nobj or any(not math.isfinite(v) for v in reference):
            raise ValueError("invalid Pareto reference point")
        acquisition = qLogNoisyExpectedHypervolumeImprovement(model=model, ref_point=reference,
            X_baseline=xn, sampler=SobolQMCNormalSampler(torch.Size([128])), X_pending=pending,
            objective=IdentityMCMultiOutputObjective(outcomes=list(range(nobj))),
            constraints=constraints or None, prune_baseline=True)
    standard = torch.zeros_like(bounds)
    standard[1] = 1
    candidates, _ = optimize_acqf(acq_function=acquisition, bounds=standard, q=1,
        num_restarts=10, raw_samples=512, options={"batch_limit": 5, "maxiter": 200}, sequential=True)
    result = unnormalize(candidates.detach(), bounds)
    if not torch.isfinite(result).all() or not ((result >= bounds[0]) & (result <= bounds[1])).all():
        raise ValueError("invalid proposal result")
    return result.tolist()


if __name__ == "__main__":
    core.write_json(Path(sys.argv[2]), {"candidate": propose(core.read_json(sys.argv[1], 1 << 20))})
