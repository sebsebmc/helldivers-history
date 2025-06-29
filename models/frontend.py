from datetime import datetime
from pydantic import BaseModel, ConfigDict, Field, computed_field
from typing import Dict, List, Optional, Union

class Statistics(BaseModel):
    model_config = ConfigDict(populate_by_name=True)
    missions_won: Optional[int] = Field(None, alias='missionsWon')
    missions_lost: Optional[int] = Field(None, alias='missionsLost')
    mission_time: Optional[int] = Field(None, alias='missionTime')
    terminid_kills: Optional[int] = Field(None, alias='terminidKills')
    automaton_kills: Optional[int] = Field(None, alias='automatonKills')
    illuminate_kills: Optional[int] = Field(None, alias='illuminateKills')
    bullets_fired: Optional[int] = Field(None, alias='bulletsFired')
    bullets_hit: Optional[int] = Field(None, alias='bulletsHit')
    time_played: Optional[int] = Field(None, alias='timePlayed')
    deaths: Optional[int] = None
    revives: Optional[int] = None
    friendlies: Optional[int] = None
    mission_success_rate: Optional[int] = Field(None, alias='missionSuccessRate')
    accuracy: Optional[int] = None
    player_count: int = Field(None, alias='playerCount')

class Position(BaseModel):
    x: float
    y: float

class Planet(BaseModel):
    position: Position
    index: int
    name: Dict[str,str]
    sector: str
    waypoints: List[int]
    disabled: bool
    regen_per_second: float
    current_owner: str
    initial_owner: str
    health: int
    max_health: int
    statistics: Optional[Statistics]
    attacking: Optional[List[int]]

    @computed_field
    @property
    def liberation(self) -> float:
        if self.current_owner == "Humans":
            return (self.health / self.max_health) * 100
        else:
            return 100.0 - ((self.health / self.max_health) * 100)

class Defense(BaseModel):
    id: int
    faction: str
    type: int = 1 #
    start_time: int
    end_time: int
    health: int
    max_health: int
    joint_operation_ids: List[int]
    planet: Planet
    
    @computed_field
    @property
    def liberation(self) -> float:
        return 100.0 - ((self.health / self.max_health) * 100)

class Reward(BaseModel):
    type: int
    amount: int

class Task(BaseModel):
    type: int
    values: List[int]
    value_types: List[int]

class Assignment(BaseModel):
    id: int
    # translated strings should be keyed on language
    title: Dict[str,Optional[str]]
    briefing: Dict[str,Optional[str]]
    description: Dict[str,Optional[str]]
    tasks: List[Task]
    progress: List[int]
    expiration: int
    reward: Optional[Reward]

class WarDetails(BaseModel):
    start_time: int
    end_time: Optional[int]
    now: int
    factions: List[str]
    impact_multiplier: float
    # We didnt always have the stats
    statistics: Optional[Statistics]

class Campaign(BaseModel):
    id: int
    planet: Planet
    type: int
    count: int

class Dispatch(BaseModel):
    id: int
    message: Dict[str, Optional[str]]
    title:  Dict[str, str]
    faction: Optional[str] = None

class CurrentStatus(BaseModel):
    # union type for now in case we get more types later
    events: List[Union[Defense]]
    planets: List[Planet]
    # There aren't always M.O.s  and we dont have them in the old data
    assignments: List[Assignment]
    war: WarDetails
    active: List[Campaign]
    dispatches: List[Dispatch]
    snapshot_at: int
