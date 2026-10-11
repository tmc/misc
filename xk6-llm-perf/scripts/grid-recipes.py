#!/usr/bin/env python3
"""Plan and post-process bounded, retained AIPerf-compatible grid recipes."""
import argparse
import importlib.util
import itertools
import math
from pathlib import Path
import sys
import stat as filemode

spec = importlib.util.spec_from_file_location('campaign_core', Path(__file__).with_name('optuna-campaign.py'))
core = importlib.util.module_from_spec(spec)
spec.loader.exec_module(core)
RECIPES = ('concurrency-ramp', 'prefill-ttft-curve', 'decode-itl-curve', 'pareto-sweep')


def integer(value):
    if type(value) is not int or not 1 <= value <= 1000000:
        raise ValueError('axis must be a positive bounded integer')
    return value


def logspace(low, high, steps):
    integer(low), integer(high), integer(steps)
    if high <= low or not 2 <= steps <= 64:
        raise ValueError('invalid log grid')
    values = {round(math.exp(math.log(low) + (math.log(high)-math.log(low))*i/(steps-1))) for i in range(steps)}
    return sorted(values | {low, high})


def plan(config):
    if not isinstance(config, dict) or config.get('recipe') not in RECIPES or set(config)-{'recipe', 'streaming', 'concurrency', 'isl', 'osl', 'pairs', 'threshold', 'metric', 'stat'}:
        raise ValueError('invalid recipe configuration')
    name = config['recipe']
    allowed={'concurrency-ramp':{'concurrency','threshold','metric','stat'},'prefill-ttft-curve':{'isl'},'decode-itl-curve':{'concurrency','osl'},'pareto-sweep':{'pairs','concurrency'}}[name] | {'recipe','streaming'}
    if set(config)-allowed: raise ValueError('unsupported recipe override')
    for key in ('metric','stat'):
        if key in config and (not isinstance(config[key],str) or not 1 <= len(config[key]) <= 256):
            raise ValueError('invalid metric selector')
    if 'threshold' in config: number(config['threshold'])
    if type(config.get('streaming')) is not bool or name != 'concurrency-ramp' and not config['streaming']:
        raise ValueError('recipe requires explicit streaming policy')
    def axis(key, default):
        value = config.get(key, default)
        if not isinstance(value, list) or len(value) != 3:
            raise ValueError('axis requires [min,max,steps]')
        return logspace(*value)
    cs = axis('concurrency', [1, 200 if name == 'decode-itl-curve' else 1000, 6 if name == 'decode-itl-curve' else 8]) if name != 'prefill-ttft-curve' and name != 'pareto-sweep' else [1]
    if name == 'concurrency-ramp':
        cells = [{'CONCURRENCY': c} for c in cs]
    elif name == 'prefill-ttft-curve':
        cells = [{'CONCURRENCY': 1, 'ISL': i} for i in axis('isl', [256,32768,8])]
    elif name == 'decode-itl-curve':
        cells = [{'CONCURRENCY': c, 'OSL': o} for c,o in itertools.product(cs, axis('osl', [64,1024,4]))]
    else:
        pairs, cs = config.get('pairs'), config.get('concurrency', [1,4,16,64,256])
        if not isinstance(pairs,list) or not pairs or not isinstance(cs,list) or not cs:
            raise ValueError('paired shapes and concurrencies required')
        if len(pairs)*len(cs)>4096: raise ValueError('grid exceeds cell limit')
        for pair in pairs:
            if not isinstance(pair,list) or len(pair)!=2:
                raise ValueError('invalid paired shape')
            for value in pair: integer(value)
        for c in cs: integer(c)
        cells = [{'CONCURRENCY':c,'ISL':i,'OSL':o} for (i,o),c in itertools.product(pairs,cs)]
    if not 2 <= len(cells) <= 4096 or len({core.sha(cell) for cell in cells}) != len(cells):
        raise ValueError('grid must contain 2..4096 distinct cells')
    return cells


def number(value):
    if isinstance(value,bool) or not isinstance(value,(int,float)) or not math.isfinite(value) or value < 0:
        raise ValueError('metric must be finite and nonnegative')
    return float(value)


def retained(directory, cell, metrics):
    directory=directory.resolve(strict=True)
    manifest_path=directory/'campaign.json'
    if not filemode.S_ISREG(manifest_path.lstat().st_mode) or manifest_path.stat().st_size > 64<<20:
        raise ValueError('manifest must be a bounded regular file')
    manifest=core.read_json(manifest_path)
    candidates=manifest.get('candidates')
    if not isinstance(candidates,list) or len(candidates)!=1:
        raise ValueError('single retained candidate required')
    attempts=candidates[0].get('attempts')
    if not isinstance(attempts,list) or not 1 <= len(attempts) <= 10000:
        raise ValueError('invalid attempt count')
    for attempt in attempts:
        relative=Path(attempt['directory'])/'summary.json'
        path=directory/relative
        if relative.is_absolute() or '..' in relative.parts or path.resolve(strict=True).parent != (directory/attempt['directory']).absolute() or not filemode.S_ISREG(path.lstat().st_mode) or path.stat().st_size > 64<<20:
            raise ValueError('summary must be a bounded regular contained file')
    parameters = {key:str(value) for key,value in cell.items()}
    receipt = core.observed(directory, parameters)
    if receipt['state'] != 'COMPLETE' or any(v > 0 for v in receipt['constraint_violations']):
        raise ValueError('cell is not complete and feasible')
    manifest = core.read_json(directory/'campaign.json')
    attempts = manifest['candidates'][0]['attempts']
    if not 1 <= len(attempts) <= 10000:
        raise ValueError('invalid attempt count')
    values = []
    for metric,stat in metrics:
        samples=[]
        for attempt in attempts:
            if attempt.get('error') or attempt.get('exit_code') != 0 or not attempt.get('summary_sha256'):
                raise ValueError('failed or unhashed attempt')
            summary=core.read_json(directory/attempt['directory']/'summary.json')
            block=summary.get('metrics',{}).get(metric,{})
            samples.append(number(block.get('values',block).get(stat)))
        values.append(sum(v/len(samples) for v in samples))
    return values, receipt['artifact_sha256']


def process(config, cells):
    expected=plan(config)
    if not isinstance(cells,list) or len(cells)>4096:
        raise ValueError('invalid retained cell list')
    name=config['recipe']
    metric,stat=config.get('metric','llm_request_latency'),config.get('stat','p(99)')
    if name=='concurrency-ramp' and metric in ('llm_ttft','llm_inter_token_latency') and not config['streaming']:
        raise ValueError('streaming metric requires streaming')
    metrics=[(metric,stat)] if name=='concurrency-ramp' else [('llm_ttft','avg')] if name=='prefill-ttft-curve' else [('llm_inter_token_latency','avg')] if name=='decode-itl-curve' else [('llm_ttft','p(95)'),('llm_completion_tokens','rate')]
    rows,seen=[],set()
    for entry in cells:
        if not isinstance(entry,dict) or set(entry)!={'parameters','directory'} or core.sha(entry['parameters']) not in {core.sha(cell) for cell in expected}:
            raise ValueError('cell is outside recipe plan')
        key=core.sha(entry['parameters'])
        if key in seen: raise ValueError('duplicate cell')
        seen.add(key)
        values,hashes=retained(Path(entry['directory']),entry['parameters'],metrics)
        rows.append({'parameters':entry['parameters'],'values':values,'artifact_sha256':hashes})
    if name!='decode-itl-curve' and len(rows)!=len(expected):
        raise ValueError('incomplete grid')
    if len(rows)<2: raise ValueError('at least two retained cells required')
    rows.sort(key=lambda r:tuple(r['parameters'].get(k,0) for k in ('CONCURRENCY','ISL','OSL')))
    result={'schema_version':1,'recipe':name,'metric_mapping':metrics,'rows':rows,'planned_cells':expected}
    if name=='concurrency-ramp':
        threshold=number(config.get('threshold',.2)); baseline=rows[0]['values'][0]
        if baseline<=0: raise ValueError('baseline must be positive')
        cutoff=baseline*(1+threshold)
        if not math.isfinite(cutoff): raise ValueError('cutoff overflow')
        knee=next((r for r in rows[1:] if r['values'][0]>cutoff),None)
        result.update(baseline_concurrency=rows[0]['parameters']['CONCURRENCY'],baseline=baseline,threshold=threshold,knee_concurrency=knee['parameters']['CONCURRENCY'] if knee else None)
    elif name=='prefill-ttft-curve':
        import numpy as np
        if core.importlib.metadata.version('numpy') != '2.4.6': raise ValueError('NumPy version differs from pin')
        x=np.array([r['parameters']['ISL'] for r in rows],dtype=float); y=np.array([r['values'][0] for r in rows])
        def fit(degree):
            coefficients=np.polyfit(x,y,degree); total=float(np.sum((y-y.mean())**2)); residual=float(np.sum((y-np.polyval(coefficients,x))**2))
            return coefficients.tolist(),0 if total==0 else 1-residual/total
        coefficients,r2=fit(1); form='linear'
        if r2<.85 and len(rows)>=3:
            alternative,score=fit(2)
            if score>r2: coefficients,r2,form=alternative,score,'quadratic'
        if not all(math.isfinite(v) for v in coefficients+[r2]): raise ValueError('nonfinite fit')
        result.update(fit_form=form,coefficients=coefficients,r_squared=r2,below_floor=r2<.85,r_squared_floor=.85)
    elif name=='decode-itl-curve':
        cs=sorted({r['parameters']['CONCURRENCY'] for r in rows}); os=sorted({r['parameters']['OSL'] for r in rows})
        index={(r['parameters']['CONCURRENCY'],r['parameters']['OSL']):r['values'][0] for r in rows}
        result['surface']={'concurrency_axis':cs,'osl_axis':os,'itl_grid':[[index.get((c,o)) for o in os] for c in cs]}
    else:
        front=[r for r in rows if not any(s['values'][0]<=r['values'][0] and s['values'][1]>=r['values'][1] and s['values']!=r['values'] for s in rows)]
        result['pareto_front']=[r['parameters'] for r in front]
    return result


def main():
    parser=argparse.ArgumentParser(description=__doc__); parser.add_argument('config'); parser.add_argument('--cells')
    args=parser.parse_args(); config=core.read_json(Path(args.config))
    output=process(config,core.read_json(Path(args.cells))) if args.cells else {'recipe':config['recipe'],'cells':plan(config)}
    print(core.json.dumps(output,allow_nan=False,sort_keys=True))

if __name__=='__main__':
    try: main()
    except (ValueError,OSError,KeyError,TypeError) as error:
        print('grid-recipes: '+str(error),file=sys.stderr); sys.exit(1)
